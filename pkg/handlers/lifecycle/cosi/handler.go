// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cosi

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultHelmReleaseName      = "cosi-controller"
	defaultHelmReleaseNamespace = "container-object-storage-system"
)

type ControllerConfig struct {
	*options.GlobalOptions

	helmAddonConfig *addons.HelmAddonConfig
}

func NewControllerConfig(globalOptions *options.GlobalOptions) *ControllerConfig {
	return &ControllerConfig{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-cosi-controller-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

func (c *ControllerConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type DefaultCOSIController struct {
	client              ctrlclient.Client
	config              *ControllerConfig
	helmChartInfoGetter *config.HelmChartGetter

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

var (
	_ commonhandlers.Named                   = &DefaultCOSIController{}
	_ lifecycle.AfterControlPlaneInitialized = &DefaultCOSIController{}
	_ lifecycle.BeforeClusterUpgrade         = &DefaultCOSIController{}
)

func New(
	c ctrlclient.Client,
	cfg *ControllerConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *DefaultCOSIController {
	return &DefaultCOSIController{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.COSIVariableName},
	}
}

func (n *DefaultCOSIController) Name() string {
	return "COSIControllerHandler"
}

func (n *DefaultCOSIController) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultCOSIController) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultCOSIController) apply(
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

	cosiVar, err := variables.Get[apivariables.COSI](varMap, n.variableName, n.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.
				Info(
					"Skipping COSI handler, cluster does not specify request COSI Controller addon deployment",
				)
			return
		}
		log.Error(
			err,
			"failed to read COSI variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read COSI variable from cluster definition: %v",
				err,
			),
		)
		return
	}

	var strategy addons.Applier
	switch cosiVar.Strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := n.helmChartInfoGetter.For(ctx, log, config.COSIController)
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
	case v1alpha1.AddonStrategyClusterResourceSet:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"strategy %q not provided for COSI", v1alpha1.AddonStrategyClusterResourceSet,
			),
		)
		return
	case "":
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("strategy not provided for COSI")
		return
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("unknown COSI addon deployment strategy %q", cosiVar.Strategy))
		return
	}

	if err := strategy.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		err = fmt.Errorf("failed to apply COSI addon: %w", err)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
