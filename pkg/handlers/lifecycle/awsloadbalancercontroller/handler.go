// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package awsloadbalancercontroller

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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	// Abbreviate name to be <63 characters with the UUID suffix.
	defaultHelmReleaseName      = "aws-lb-controller"
	defaultHelmReleaseNamespace = "kube-system"
)

type ControllerConfig struct {
	*options.GlobalOptions

	helmAddonConfig *addons.HelmAddonConfig
}

func NewControllerConfig(globalOptions *options.GlobalOptions) *ControllerConfig {
	return &ControllerConfig{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-aws-load-balancer-controller-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

func (c *ControllerConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type DefaultAWSLoadBalancerController struct {
	client              ctrlclient.Client
	config              *ControllerConfig
	helmChartInfoGetter *config.HelmChartGetter

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

var (
	_ commonhandlers.Named                   = &DefaultAWSLoadBalancerController{}
	_ lifecycle.AfterControlPlaneInitialized = &DefaultAWSLoadBalancerController{}
	_ lifecycle.BeforeClusterUpgrade         = &DefaultAWSLoadBalancerController{}
)

func New(
	c ctrlclient.Client,
	cfg *ControllerConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *DefaultAWSLoadBalancerController {
	return &DefaultAWSLoadBalancerController{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", "loadBalancerController"},
	}
}

func (n *DefaultAWSLoadBalancerController) Name() string {
	return "AWSLoadBalancerControllerHandler"
}

func (n *DefaultAWSLoadBalancerController) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultAWSLoadBalancerController) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultAWSLoadBalancerController) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	// For now, always enable the AWS Load Balancer Controller
	// FIXME: Add proper variable checking when APIs are added
	if provider := utils.GetProvider(cluster); provider != "eks" && provider != "aws" {
		log.V(5).Info("Skipping AWS Load Balancer Controller handler, not an EKS or AWS cluster", provider)
		return
	}

	log.Info("Installing AWS Load Balancer Controller addon")

	helmChart, err := n.helmChartInfoGetter.For(ctx, log, config.AWSLoadBalancerController)
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

	strategy := addons.NewHelmAddonApplier(
		n.config.helmAddonConfig,
		n.client,
		helmChart,
	)

	if err := strategy.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		err = fmt.Errorf("failed to apply AWS Load Balancer Controller addon: %w", err)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
