// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

type addonStrategy interface {
	apply(context.Context, *runtimehooksv1.AfterControlPlaneInitializedRequest, logr.Logger) error
}

type Config struct {
	crsConfig crsConfig
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.crsConfig.AddFlags(prefix+".crs", flags)
}

type DefaultClusterAutoscaler struct {
	client ctrlclient.Client
	config *Config

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

var (
	_ commonhandlers.Named                   = &DefaultClusterAutoscaler{}
	_ lifecycle.AfterControlPlaneInitialized = &DefaultClusterAutoscaler{}
)

func New(
	c ctrlclient.Client,
	cfg *Config,
) *DefaultClusterAutoscaler {
	return &DefaultClusterAutoscaler{
		client:       c,
		config:       cfg,
		variableName: clusterconfig.MetaVariableName,
		variablePath: []string{"addons", v1alpha1.ClusterAutoscalerVariableName},
	}
}

func (n *DefaultClusterAutoscaler) Name() string {
	return "ClusterAutoscalerHandler"
}

func (n *DefaultClusterAutoscaler) AfterControlPlaneInitialized(
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

	cniVar, found, err := variables.Get[v1alpha1.ClusterAutoscaler](
		varMap,
		n.variableName,
		n.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read cluster-autoscaler variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read cluster-autoscaler variable from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found {
		log.Info(
			"Skipping cluster-autoscaler handler, cluster does not specify request cluster-autoscaler addon deployment",
		)
		return
	}

	var strategy addonStrategy
	switch cniVar.Strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: n.config.crsConfig,
			client: n.client,
		}
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("unknown cluster-autoscaler addon deployment strategy %q", cniVar.Strategy),
		)
		return
	}

	if err := strategy.apply(ctx, req, log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
