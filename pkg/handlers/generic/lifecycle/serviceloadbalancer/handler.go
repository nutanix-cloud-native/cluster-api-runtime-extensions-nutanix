// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serviceloadbalancer

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

type ServiceLoadBalancerProvider interface {
	Apply(
		ctx context.Context,
		cluster *clusterv1.Cluster,
		log logr.Logger,
	) error
}

type ServiceLoadBalancerHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]ServiceLoadBalancerProvider
}

var (
	_ commonhandlers.Named                   = &ServiceLoadBalancerHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &ServiceLoadBalancerHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]ServiceLoadBalancerProvider,
) *ServiceLoadBalancerHandler {
	return &ServiceLoadBalancerHandler{
		client:          c,
		variableName:    v1alpha1.ClusterConfigVariableName,
		variablePath:    []string{"addons", v1alpha1.ServiceLoadBalancerVariableName},
		ProviderHandler: handlers,
	}
}

func (s *ServiceLoadBalancerHandler) Name() string {
	return "ServiceLoadBalancerHandler"
}

func (s *ServiceLoadBalancerHandler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	s.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (s *ServiceLoadBalancerHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	s.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
	if resp.Status == runtimehooksv1.ResponseStatusFailure {
		resp.SetRetryAfterSeconds(lifecycle.BeforeClusterUpgradeRetryAfterSeconds)
	}
}

func (s *ServiceLoadBalancerHandler) apply(
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
	slb, err := variables.Get[v1alpha1.ServiceLoadBalancer](
		varMap,
		s.variableName,
		s.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info(
					"Skipping ServiceLoadBalancer, field is not specified",
					"error",
					err,
				)
			return
		}
		log.Error(
			err,
			"failed to read ServiceLoadBalancer provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read ServiceLoadBalancer provider from cluster definition: %v",
				err,
			),
		)
		return
	}

	handler, ok := s.ProviderHandler[slb.Provider]
	if !ok {
		err = fmt.Errorf("unknown ServiceLoadBalancer Provider")
		log.Error(err, "provider", slb.Provider)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("%s %s", err, slb.Provider),
		)
		return
	}

	log.Info(fmt.Sprintf("Deploying ServiceLoadBalancer provider %s", slb.Provider))
	err = handler.Apply(
		ctx,
		cluster,
		log,
	)
	if err != nil {
		log.Error(
			err,
			fmt.Sprintf(
				"failed to deploy ServiceLoadBalancer provider %s",
				slb.Provider,
			),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"failed to deploy ServiceLoadBalancer provider: %v",
				err,
			),
		)
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	resp.SetMessage(
		fmt.Sprintf(
			"deployed ServiceLoadBalancer provider %s",
			slb.Provider),
	)
}
