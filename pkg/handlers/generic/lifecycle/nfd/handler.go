// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nfd

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type addonStrategy interface {
	apply(
		context.Context,
		*runtimehooksv1.AfterControlPlaneInitializedRequest,
		string,
		logr.Logger,
	) error
}

type Config struct {
	*options.GlobalOptions

	crsConfig       crsConfig
	helmAddonConfig helmAddonConfig
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.crsConfig.AddFlags(prefix+".crs", flags)
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type DefaultNFD struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

var (
	_ commonhandlers.Named                   = &DefaultNFD{}
	_ lifecycle.AfterControlPlaneInitialized = &DefaultNFD{}
)

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *DefaultNFD {
	return &DefaultNFD{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        clusterconfig.MetaVariableName,
		variablePath:        []string{"addons", v1alpha1.NFDVariableName},
	}
}

func (n *DefaultNFD) Name() string {
	return "NFDHandler"
}

func (n *DefaultNFD) AfterControlPlaneInitialized(
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

	cniVar, found, err := variables.Get[v1alpha1.NFD](varMap, n.variableName, n.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read NFD variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read NFD variable from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found {
		log.Info("Skipping NFD handler, cluster does not specify request NFDaddon deployment")
		return
	}

	var strategy addonStrategy
	switch cniVar.Strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: n.config.crsConfig,
			client: n.client,
		}
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := n.helmChartInfoGetter.For(ctx, log, config.NFD)
		if err != nil {
			log.Error(
				err,
				"failed to get configmap with helm settings",
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf("failed to get configration to create helm addon: %v",
					err,
				),
			)
			return
		}
		strategy = helmAddonStrategy{
			config:    n.config.helmAddonConfig,
			client:    n.client,
			helmChart: helmChart,
		}
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("unknown NFD addon deployment strategy %q", cniVar.Strategy))
		return
	}

	if err := strategy.apply(ctx, req, n.config.DefaultsNamespace(), log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
