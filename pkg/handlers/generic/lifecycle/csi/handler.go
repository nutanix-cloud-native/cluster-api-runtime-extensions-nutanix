// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
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
	c.handle(ctx, &req.Cluster, &resp.CommonResponse)
}

func (c *CSIHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	c.handle(ctx, &req.Cluster, &resp.CommonResponse)
}

func (c *CSIHandler) OnClusterSpecUpdated(
	ctx context.Context,
	cluster *clusterv1.Cluster,
) error {
	if err := c.apply(ctx, cluster); err != nil {
		return fmt.Errorf("failed to apply CSI: %w", err)
	}

	return nil
}

func (c *CSIHandler) handle(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	if err := c.apply(ctx, cluster); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (c *CSIHandler) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
) error {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)
	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	csi, err := variables.Get[apivariables.CSI](
		varMap,
		c.variableName,
		c.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Skipping CSI handler, the cluster does not define the CSI variable")
			return nil
		}
		return fmt.Errorf("failed to read the CSI variable from the cluster: %w", err)
	}

	// This is defensive, because the API validation requires at least one provider.
	if len(csi.Providers) == 0 {
		return errors.New("the list of CSI providers must include at least one provider")
	}

	if err := validateDefaultStorage(csi); err != nil {
		return fmt.Errorf("failed to validate default: %w", err)
	}

	// There's a 1:N mapping of infra to CSI providers. The user chooses the providers.
	for providerName, provider := range csi.Providers {
		handler, ok := c.ProviderHandler[providerName]
		if !ok {
			return fmt.Errorf("CSI provider %q is unknown", providerName)
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
			return fmt.Errorf("failed to deploy %q CSI driver: %w", providerName, err)
		}
	}

	return nil
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
