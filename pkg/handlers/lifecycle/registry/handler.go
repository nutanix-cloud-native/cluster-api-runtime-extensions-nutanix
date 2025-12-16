// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

type RegistryProvider interface {
	Setup(
		ctx context.Context,
		registryVar v1alpha1.RegistryAddon,
		cluster *clusterv1.Cluster,
		log logr.Logger,
	) error
	Apply(
		ctx context.Context,
		registryVar v1alpha1.RegistryAddon,
		cluster *clusterv1.Cluster,
		log logr.Logger,
	) error
	Cleanup(
		ctx context.Context,
		registryVar v1alpha1.RegistryAddon,
		cluster *clusterv1.Cluster,
		log logr.Logger,
	) error
}

type RegistryHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]RegistryProvider
}

var (
	_ commonhandlers.Named                   = &RegistryHandler{}
	_ lifecycle.BeforeClusterCreate          = &RegistryHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &RegistryHandler{}
	_ lifecycle.BeforeClusterUpgrade         = &RegistryHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]RegistryProvider,
) *RegistryHandler {
	return &RegistryHandler{
		client:          c,
		variableName:    v1alpha1.ClusterConfigVariableName,
		variablePath:    []string{"addons", v1alpha1.RegistryAddonVariableName},
		ProviderHandler: handlers,
	}
}

func (r *RegistryHandler) Name() string {
	return "RegistryHandler"
}

func (r *RegistryHandler) BeforeClusterCreate(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterCreateRequest,
	resp *runtimehooksv1.BeforeClusterCreateResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	r.setup(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (r *RegistryHandler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	r.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (r *RegistryHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	r.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (r *RegistryHandler) setup(
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
	registryVar, err := variables.Get[v1alpha1.RegistryAddon](
		varMap,
		r.variableName,
		r.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info(
					"Skipping RegistryAddon, field is not specified",
					"error",
					err,
				)
			return
		}
		log.Error(
			err,
			"failed to read RegistryAddon provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read RegistryAddon provider from cluster definition: %v",
				err,
			),
		)
		return
	}

	handler, ok := r.ProviderHandler[registryVar.Provider]
	if !ok {
		err = fmt.Errorf("unknown RegistryAddon Provider")
		log.Error(err, "provider", registryVar.Provider)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("%s %s", err, registryVar.Provider),
		)
		return
	}

	log.Info(fmt.Sprintf("Setting up RegistryAddon provider prerequisites %s", registryVar.Provider))
	err = handler.Setup(
		ctx,
		registryVar,
		cluster,
		log,
	)
	if err != nil {
		log.Error(
			err,
			fmt.Sprintf(
				"failed to set up RegistryAddon provider prerequisites %s",
				registryVar.Provider,
			),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"failed to set up RegistryAddon provider prerequisites: %v",
				err,
			),
		)
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	resp.SetMessage(
		fmt.Sprintf(
			"Set up RegistryAddon provider prerequisites %s",
			registryVar.Provider,
		),
	)
}

func (r *RegistryHandler) apply(
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
	registryVar, err := variables.Get[v1alpha1.RegistryAddon](
		varMap,
		r.variableName,
		r.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info(
					"Skipping RegistryAddon, field is not specified",
					"error",
					err,
				)
			return
		}
		log.Error(
			err,
			"failed to read RegistryAddon provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read RegistryAddon provider from cluster definition: %v",
				err,
			),
		)
		return
	}

	handler, ok := r.ProviderHandler[registryVar.Provider]
	if !ok {
		err = fmt.Errorf("unknown RegistryAddon Provider")
		log.Error(err, "provider", registryVar.Provider)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("%s %s", err, registryVar.Provider),
		)
		return
	}

	log.Info(fmt.Sprintf("Deploying RegistryAddon provider %s", registryVar.Provider))
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
				"failed to deploy RegistryAddon provider %s",
				registryVar.Provider,
			),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"failed to deploy RegistryAddon provider: %v",
				err,
			),
		)
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	resp.SetMessage(
		fmt.Sprintf(
			"Deployed RegistryAddon provider %s",
			registryVar.Provider,
		),
	)
}

func (r *RegistryHandler) BeforeClusterDelete(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	cluster := &req.Cluster

	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	registryVar, err := variables.Get[v1alpha1.RegistryAddon](
		varMap,
		r.variableName,
		r.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info(
					"Skipping RegistryAddon, field is not specified",
					"error",
					err,
				)
			return
		}
		log.Error(
			err,
			"failed to read RegistryAddon provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read RegistryAddon provider from cluster definition: %v",
				err,
			),
		)
		return
	}

	handler, ok := r.ProviderHandler[registryVar.Provider]
	if !ok {
		err = fmt.Errorf("unknown RegistryAddon Provider")
		log.Error(err, "provider", registryVar.Provider)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("%s %s", err, registryVar.Provider),
		)
		return
	}

	log.Info(fmt.Sprintf("Clean up RegistryAddon provider %s", registryVar.Provider))
	err = handler.Cleanup(
		ctx,
		registryVar,
		cluster,
		log,
	)
	if err != nil {
		log.Error(
			err,
			fmt.Sprintf(
				"failed to clean up RegistryAddon provider %s",
				registryVar.Provider,
			),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"failed to clean up RegistryAddon provider: %v",
				err,
			),
		)
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
