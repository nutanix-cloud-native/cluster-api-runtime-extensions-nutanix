// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
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
	csiProviders, found, err := variables.Get[v1alpha1.CSI](
		varMap,
		c.variableName,
		c.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read CSI provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CSI provider from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found || csiProviders.Providers == nil || len(csiProviders.Providers) == 0 {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping CSI handler, no providers given in %v",
				csiProviders,
			),
		)
		return
	}
	if len(csiProviders.Providers) == 1 &&
		len(csiProviders.Providers[0].StorageClassConfig) == 1 &&
		csiProviders.DefaultStorage == nil {
		csiProviders.DefaultStorage = &v1alpha1.DefaultStorage{
			ProviderName:           csiProviders.Providers[0].Name,
			StorageClassConfigName: csiProviders.Providers[0].StorageClassConfig[0].Name,
		}
	}

	for _, provider := range csiProviders.Providers {
		handler, ok := c.ProviderHandler[provider.Name]
		if !ok {
			log.V(4).Info(
				fmt.Sprintf(
					"Skipping CSI handler, for provider given in %s. Provider handler not given.",
					provider.Name,
				),
			)
			continue
		}
		log.Info(fmt.Sprintf("Creating CSI provider %s", provider.Name))
		err = handler.Apply(
			ctx,
			provider,
			csiProviders.DefaultStorage,
			req,
		)
		if err != nil {
			log.Error(
				err,
				fmt.Sprintf(
					"failed to delpoy %s CSI driver",
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
