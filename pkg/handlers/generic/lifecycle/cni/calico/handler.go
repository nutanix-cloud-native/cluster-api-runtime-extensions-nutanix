// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

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

type CNIConfig struct {
	crsConfig   crsConfig
	caaphConfig caaphConfig
}

type CalicoCNI struct {
	client ctrlclient.Client
	config *CNIConfig

	variableName string
	variablePath []string
}

func (c *CNIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.crsConfig.AddFlags(prefix+".crs", flags)
	c.crsConfig.AddFlags(prefix+".caaph", flags)
}

var (
	_ commonhandlers.Named                   = &CalicoCNI{}
	_ lifecycle.AfterControlPlaneInitialized = &CalicoCNI{}
)

func New(
	c ctrlclient.Client,
	cfg *CNIConfig,
) *CalicoCNI {
	return &CalicoCNI{
		client:       c,
		config:       cfg,
		variableName: clusterconfig.MetaVariableName,
		variablePath: []string{"addons", v1alpha1.CNIVariableName},
	}
}

func (s *CalicoCNI) Name() string {
	return "CalicoCNI"
}

func (s *CalicoCNI) AfterControlPlaneInitialized(
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

	cniVar, found, err := variables.Get[v1alpha1.CNI](varMap, s.variableName, s.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read CNI provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CNI provider from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found {
		log.V(4).
			Info("Skipping Calico CNI handler, cluster does not specify request CNI addon deployment")
		return
	}
	if cniVar.Provider != v1alpha1.CNIProviderCalico {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping Calico CNI handler, cluster does not specify %q as value of CNI provider variable",
				v1alpha1.CNIProviderCalico,
			),
		)
		return
	}

	var strategy addonStrategy
	switch cniVar.Strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: s.config.crsConfig,
			client: s.client,
		}
	case v1alpha1.AddonStrategyClusterAPIAddonProviderHelm:
		strategy = caaphStrategy{
			config: s.config.caaphConfig,
			client: s.client,
		}
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("unknown CNI addon deployment strategy %q", cniVar.Strategy))
		return
	}

	if err := strategy.apply(ctx, req, log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
