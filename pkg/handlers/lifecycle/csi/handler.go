// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

const (
	variableRootName = "csi"
)

type CSIProvider interface {
	Apply(
		context.Context,
		v1alpha1.CSIProvider,
		v1alpha1.DefaultStorage,
		*clusterv1.Cluster,
		logr.Logger,
	) error
}

type CSIHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]CSIProvider
}

var (
	_ commonhandlers.Named                   = &CSIHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &CSIHandler{}
	_ lifecycle.BeforeClusterUpgrade         = &CSIHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]CSIProvider,
) *CSIHandler {
	return &CSIHandler{
		client:          c,
		variableName:    v1alpha1.ClusterConfigVariableName,
		variablePath:    []string{"addons", variableRootName},
		ProviderHandler: handlers,
	}
}

func (c *CSIHandler) Name() string {
	return "CSIHandler"
}

func (c *CSIHandler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	cluster, err := capiutils.ConvertV1Beta1ClusterToV1Beta2(&req.Cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to convert cluster: %v", err))
		return
	}
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *CSIHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	cluster, err := capiutils.ConvertV1Beta1ClusterToV1Beta2(&req.Cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to convert cluster: %v", err))
		return
	}
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *CSIHandler) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)
	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	csi, err := variables.Get[apivariables.CSI](
		varMap,
		c.variableName,
		c.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Skipping CSI handler, the cluster does not define the CSI variable")
			return
		}
		msg := "failed to read the CSI variable from the cluster"
		log.Error(err, msg)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("%s: %v", msg, err))
		return
	}

	// This is defensive, because the API validation requires at least one provider.
	if len(csi.Providers) == 0 {
		msg := "The list of CSI providers must include at least one provider"
		log.Error(nil, msg)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(msg)
		return
	}

	if err := validateDefaultStorage(csi); err != nil {
		log.Error(err, "")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	// There's a 1:N mapping of infra to CSI providers. The user chooses the providers.
	for providerName, provider := range csi.Providers {
		handler, ok := c.ProviderHandler[providerName]
		if !ok {
			log.Error(
				nil,
				"CSI provider is unknown",
				"name",
				providerName,
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf("CSI provider %q is unknown", providerName),
			)
			return
		}
		log.Info(fmt.Sprintf("Creating CSI provider %s", providerName))
		err = handler.Apply(
			ctx,
			provider,
			csi.DefaultStorage,
			cluster,
			log,
		)
		if err != nil {
			log.Error(
				err,
				fmt.Sprintf(
					"failed to deploy %s CSI driver",
					providerName,
				),
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf(
					"failed to deploy CSI driver: %v",
					err,
				),
			)
		}
	}
}

// Verify that the default storage references a defined storage class config.
func validateDefaultStorage(csi apivariables.CSI) error {
	if provider, providerFound := csi.Providers[csi.DefaultStorage.Provider]; providerFound {
		_, scFound := provider.StorageClassConfigs[csi.DefaultStorage.StorageClassConfig]
		if scFound {
			return nil
		}
	}

	return fmt.Errorf(
		"the DefaultStorage StorageClassConfig name must match a configured StorageClassConfig",
	)
}
