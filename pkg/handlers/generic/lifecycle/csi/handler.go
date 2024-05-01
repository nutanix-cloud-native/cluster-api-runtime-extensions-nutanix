// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	variableRootName = "csi"
)

type CSIProvider interface {
	Apply(
		context.Context,
		v1alpha1.CSIProvider,
		*v1alpha1.DefaultStorage,
		*runtimehooksv1.AfterControlPlaneInitializedRequest,
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
)

func New(
	c ctrlclient.Client,
	handlers map[string]CSIProvider,
) *CSIHandler {
	return &CSIHandler{
		client:          c,
		variableName:    clusterconfig.MetaVariableName,
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
	clusterKey := ctrlclient.ObjectKeyFromObject(&req.Cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)
	varMap := variables.ClusterVariablesToVariablesMap(req.Cluster.Spec.Topology.Variables)
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	csi, err := variables.Get[v1alpha1.CSI](
		varMap,
		c.variableName,
		c.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.Info("Skipping CSI handler, the cluster does not define the CSI variable")
			return
		}
		log.Error(
			err,
			"failed to read the CSI variable from the cluster",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read the CSI variable from the cluster: %v",
				err,
			),
		)
		return
	}

	if len(csi.Providers) == 0 {
		log.Error(
			err,
			"The list of CSI providers must include at least one provider.",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("the list of CSI providers must include at least one provider")
		return
	}

	for _, provider := range csi.Providers {
		if len(provider.StorageClassConfig) == 0 {
			log.Error(
				err,
				"The CSI provider must configure at least one storage class.",
				"name",
				provider.Name,
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf("the CSI provider %q must configure at least one storage class",
					provider.Name),
			)
			return
		}
	}

	if csi.DefaultStorage == nil {
		if len(csi.Providers) > 1 {
			log.Error(
				err,
				"A CSI configuration with two or more providers must configure the default storage.",
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				"CSI configuration with two or more providers must configure the default storage",
			)
			return
		}
		// When there is one provider, we can derive the default storage configuration.
		csi.DefaultStorage = &v1alpha1.DefaultStorage{
			ProviderName:           csi.Providers[0].Name,
			StorageClassConfigName: csi.Providers[0].StorageClassConfig[0].Name,
		}
	}

	// There's a 1:N mapping of infra to CSI providers. The user chooses the provider.
	for _, provider := range csi.Providers {
		handler, ok := c.ProviderHandler[provider.Name]
		if !ok {
			log.Error(
				err,
				"CSI provider is unknown",
				"name",
				provider.Name,
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf("CSI provider %q is unknown", provider.Name),
			)
			return
		}
		log.Info(fmt.Sprintf("Creating CSI provider %s", provider.Name))
		err = handler.Apply(
			ctx,
			provider,
			csi.DefaultStorage,
			req,
			log,
		)
		if err != nil {
			log.Error(
				err,
				fmt.Sprintf(
					"failed to deploy %s CSI driver",
					provider.Name,
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
