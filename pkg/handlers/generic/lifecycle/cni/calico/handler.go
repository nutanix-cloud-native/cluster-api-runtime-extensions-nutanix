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

type CNIConfig struct {
	*options.GlobalOptions

	crsConfig       crsConfig
	helmAddonConfig helmAddonConfig
}

func (c *CNIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.crsConfig.AddFlags(prefix+".crs", flags)
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type CalicoCNI struct {
	client              ctrlclient.Client
	config              *CNIConfig
	helmChartInfoGetter *config.HelmChartGetter

	variableName string
	variablePath []string
}

var (
	_ commonhandlers.Named                   = &CalicoCNI{}
	_ lifecycle.AfterControlPlaneInitialized = &CalicoCNI{}
)

func New(
	c ctrlclient.Client,
	cfg *CNIConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *CalicoCNI {
	return &CalicoCNI{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        clusterconfig.MetaVariableName,
		variablePath:        []string{"addons", v1alpha1.CNIVariableName},
	}
}

func (c *CalicoCNI) Name() string {
	return "CalicoCNI"
}

func (c *CalicoCNI) AfterControlPlaneInitialized(
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

	cniVar, found, err := variables.Get[v1alpha1.CNI](varMap, c.variableName, c.variablePath...)
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
		log.
			Info(
				"Skipping Calico CNI handler, cluster does not specify request CNI addon deployment",
			)
		return
	}
	if cniVar.Provider != v1alpha1.CNIProviderCalico {
		log.Info(
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
			config: c.config.crsConfig,
			client: c.client,
		}
	case v1alpha1.AddonStrategyHelmAddon:
		// this is tigera and not calico because we deploy calico via operataor
		log.Info("fetching settings for tigera-operator-config")
		helmChart, err := c.helmChartInfoGetter.For(ctx, log, config.Tigera)
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
		log.Info(fmt.Sprintf("using settings %v to install helm chart config", helmChart))
		strategy = helmAddonStrategy{
			config:    c.config.helmAddonConfig,
			client:    c.client,
			helmChart: helmChart,
		}
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("unknown CNI addon deployment strategy %q", cniVar.Strategy))
		return
	}

	if err := strategy.apply(ctx, req, c.config.DefaultsNamespace(), log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
