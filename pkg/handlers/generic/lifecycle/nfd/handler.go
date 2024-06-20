// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nfd

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultHelmReleaseName      = "node-feature-discovery"
	defaultHelmReleaseNamespace = "node-feature-discovery"
)

type Config struct {
	*options.GlobalOptions

	crsConfig       crsConfig
	helmAddonConfig *addons.HelmAddonConfig
}

func NewConfig(globalOptions *options.GlobalOptions) *Config {
	return &Config{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-nfd-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
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
	_ lifecycle.BeforeClusterUpgrade         = &DefaultNFD{}
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
		variableName:        v1alpha1.ClusterConfigVariableName,
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
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultNFD) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultNFD) apply(
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

	cniVar, err := variables.Get[v1alpha1.NFD](varMap, n.variableName, n.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info("Skipping NFD handler, cluster does not specify request NFDaddon deployment")
			return
		}
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

	var strategy addons.Applier
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
				fmt.Sprintf("failed to get configuration to create helm addon: %v",
					err,
				),
			)
			return
		}
		strategy = addons.NewHelmAddonApplier(
			n.config.helmAddonConfig,
			n.client,
			helmChart,
		)
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("unknown NFD addon deployment strategy %q", cniVar.Strategy))
		return
	}

	if err := strategy.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		err = fmt.Errorf("failed to apply NFD addon: %w", err)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
