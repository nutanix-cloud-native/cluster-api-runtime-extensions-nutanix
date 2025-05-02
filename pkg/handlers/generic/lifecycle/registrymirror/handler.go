// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registrymirror

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

type RegistryMirrorProvider interface {
	Apply(
		ctx context.Context,
		registryVar v1alpha1.RegistryMirror,
		cluster *clusterv1.Cluster,
		log logr.Logger,
	) error
}

type RegistryMirrorHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]RegistryMirrorProvider
}

var (
	_ commonhandlers.Named                   = &RegistryMirrorHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &RegistryMirrorHandler{}
	_ lifecycle.BeforeClusterUpgrade         = &RegistryMirrorHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]RegistryMirrorProvider,
) *RegistryMirrorHandler {
	return &RegistryMirrorHandler{
		client:          c,
		variableName:    v1alpha1.ClusterConfigVariableName,
		variablePath:    []string{"addons", v1alpha1.RegistryMirrorVariableName},
		ProviderHandler: handlers,
	}
}

func (r *RegistryMirrorHandler) Name() string {
	return "RegistryMirrorHandler"
}

func (r *RegistryMirrorHandler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	r.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (r *RegistryMirrorHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	r.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (r *RegistryMirrorHandler) apply(
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
	registryVar, err := variables.Get[v1alpha1.RegistryMirror](
		varMap,
		r.variableName,
		r.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info(
					"Skipping RegistryMirror, field is not specified",
					"error",
					err,
				)
			return
		}
		log.Error(
			err,
			"failed to read RegistryMirror provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read RegistryMirror provider from cluster definition: %v",
				err,
			),
		)
		return
	}

	handler, ok := r.ProviderHandler[registryVar.Provider]
	if !ok {
		err = fmt.Errorf("unknown RegistryMirror Provider")
		log.Error(err, "provider", registryVar.Provider)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("%s %s", err, registryVar.Provider),
		)
		return
	}

	log.Info(fmt.Sprintf("Deploying RegistryMirror provider %s", registryVar.Provider))
	err = handler.Apply(
		ctx,
		registryVar,
		cluster,
		log,
	)
	if err != nil {
		log.Error(
			err,
			fmt.Sprintf(
				"failed to deploy RegistryMirror provider %s",
				registryVar.Provider,
			),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"failed to deploy RegistryMirror provider: %v",
				err,
			),
		)
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	resp.SetMessage(
		fmt.Sprintf(
			"Deployed RegistryMirror provider %s",
			registryVar.Provider,
		),
	)
}
