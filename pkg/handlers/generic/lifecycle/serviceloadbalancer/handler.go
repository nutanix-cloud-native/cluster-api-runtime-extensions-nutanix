// Copyright 2023 D2iQ, Inc. All rights reserved.
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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
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
		variableName:    clusterconfig.MetaVariableName,
		variablePath:    []string{"addons", v1alpha1.ServiceLoadBalancerVariableName},
		ProviderHandler: handlers,
	}
}

func (c *ServiceLoadBalancerHandler) Name() string {
	return "ServiceLoadBalancerHandler"
}

func (c *ServiceLoadBalancerHandler) AfterControlPlaneInitialized(
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

	slb, err := variables.Get[v1alpha1.ServiceLoadBalancer](
		varMap,
		c.variableName,
		c.variablePath...)
	if err != nil {
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

	handler, ok := c.ProviderHandler[slb.Provider]
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
		&req.Cluster,
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
