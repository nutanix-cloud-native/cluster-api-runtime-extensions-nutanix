// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type addonStrategy interface {
	apply(
		context.Context,
		*clusterv1.Cluster,
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
	_ lifecycle.BeforeClusterUpgrade         = &CalicoCNI{}
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
		variableName:        v1alpha1.ClusterConfigVariableName,
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
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *CalicoCNI) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	c.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *CalicoCNI) apply(
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

	cniVar, err := variables.Get[v1alpha1.CNI](varMap, c.variableName, c.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info(
					"Skipping Calico CNI handler, cluster does not specify request CNI addon deployment",
				)
			return
		}
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
	if cniVar.Provider != v1alpha1.CNIProviderCalico {
		log.V(5).Info(
			fmt.Sprintf(
				"Skipping Calico CNI handler, cluster does not specify %q as value of CNI provider variable",
				v1alpha1.CNIProviderCalico,
			),
		)
		return
	}

	var strategy addonStrategy
	switch ptr.Deref(cniVar.Strategy, "") {
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
				fmt.Sprintf("failed to get configuration to create helm addon: %v",
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
	case "":
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("strategy not specified for CNI addon")
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("unknown CNI addon deployment strategy %q", *cniVar.Strategy))
		return
	}

	if err := strategy.apply(ctx, cluster, c.config.DefaultsNamespace(), log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
