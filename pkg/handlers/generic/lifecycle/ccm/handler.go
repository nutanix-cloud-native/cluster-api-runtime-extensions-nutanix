// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ccm

import (
	"context"
	"fmt"
	"strings"

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
	variableRootName = "ccm"
)

type CCMProvider interface {
	Apply(
		context.Context,
		*clusterv1.Cluster,
		*apivariables.ClusterConfigSpec,
		logr.Logger,
	) error
}

type CCMHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]CCMProvider
}

var (
	_ commonhandlers.Named                   = &CCMHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &CCMHandler{}
	_ lifecycle.BeforeClusterUpgrade         = &CCMHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]CCMProvider,
) *CCMHandler {
	return &CCMHandler{
		client:          c,
		variableName:    v1alpha1.ClusterConfigVariableName,
		variablePath:    []string{"addons", variableRootName},
		ProviderHandler: handlers,
	}
}

func (c *CCMHandler) Name() string {
	return "CCMHandler"
}

func (c *CCMHandler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
	return
}

func (c *CCMHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
	if resp.Status == runtimehooksv1.ResponseStatusFailure {
		resp.SetRetryAfterSeconds(lifecycle.LifecycleRetryAfterSeconds)
	}
	return
}

func (c *CCMHandler) apply(
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

	_, err := variables.Get[v1alpha1.CCM](varMap, c.variableName, c.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Skipping CCM handler.")
			return
		}
		log.Error(
			err,
			"failed to read CCM from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CCM from cluster definition: %v",
				err,
			),
		)
		return
	}

	clusterConfigVar, err := variables.Get[apivariables.ClusterConfigSpec](
		varMap,
		v1alpha1.ClusterConfigVariableName,
	)
	if err != nil {
		log.Error(
			err,
			"failed to read clusterConfig variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read clusterConfig variable from cluster definition: %v",
				err,
			),
		)
		return
	}

	// There's a 1:1 mapping of infra to CCM provider. We derive the CCM provider from the infra.
	infraKind := cluster.Spec.InfrastructureRef.Kind
	log.Info(fmt.Sprintf("finding CCM handler for %s", infraKind))
	var handler CCMProvider
	switch {
	case strings.Contains(strings.ToLower(infraKind), v1alpha1.CCMProviderAWS):
		handler = c.ProviderHandler[v1alpha1.CCMProviderAWS]
	case strings.Contains(strings.ToLower(infraKind), v1alpha1.CCMProviderNutanix):
		handler = c.ProviderHandler[v1alpha1.CCMProviderNutanix]
	default:
		log.Info(fmt.Sprintf("No CCM handler provided for infra kind %s", infraKind))
		return
	}

	err = handler.Apply(ctx, cluster, &clusterConfigVar, log)
	if err != nil {
		log.Error(
			err,
			"failed to deploy CCM for cluster",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to deploy CCM for cluster: %v",
				err,
			),
		)
		return
	}
}
