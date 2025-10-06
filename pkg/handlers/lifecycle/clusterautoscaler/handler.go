// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	addonName = "ca"
)

type addonStrategy interface {
	apply(
		context.Context,
		*clusterv1.Cluster,
		string,
		logr.Logger,
	) error

	delete(
		context.Context,
		*clusterv1.Cluster,
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

type DefaultClusterAutoscaler struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

var (
	_ commonhandlers.Named                   = &DefaultClusterAutoscaler{}
	_ lifecycle.AfterControlPlaneInitialized = &DefaultClusterAutoscaler{}
	_ lifecycle.BeforeClusterUpgrade         = &DefaultClusterAutoscaler{}
)

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *DefaultClusterAutoscaler {
	return &DefaultClusterAutoscaler{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.ClusterAutoscalerVariableName},
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
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultClusterAutoscaler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultClusterAutoscaler) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	caVar, err := n.getCAVariable(cluster)
	if err != nil {
		log.Error(err, "failed to read cluster-autoscaler variable from cluster definition")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}
	if caVar == nil {
		log.V(5).Info(
			"Skipping cluster-autoscaler handler, cluster does not specify request cluster-autoscaler addon deployment",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		return
	}

	var strategy addonStrategy
	switch caVar.Strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: n.config.crsConfig,
			client: n.client,
		}
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := n.helmChartInfoGetter.For(
			ctx,
			log,
			config.Autoscaler,
		)
		if err != nil {
			log.Error(
				err,
				"failed to get configmap with helm settings",
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(
				fmt.Sprintf("failed to get config to create helm addon: %v",
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
	case "":
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("strategy not specified for cluster-autoscaler addon")
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("unknown cluster-autoscaler addon deployment strategy %q", caVar.Strategy),
		)
		return
	}

	if err = strategy.apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (n *DefaultClusterAutoscaler) BeforeClusterDelete(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	cluster := &req.Cluster

	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	caVar, err := n.getCAVariable(cluster)
	if err != nil {
		log.Error(err, "failed to read cluster-autoscaler variable from cluster definition")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}
	if caVar == nil {
		log.V(5).Info(
			"Skipping cluster-autoscaler before cluster delete handler, cluster does not specify request cluster-autoscaler" +
				"addon deployment",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		return
	}

	var strategy addonStrategy
	switch caVar.Strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: n.config.crsConfig,
			client: n.client,
		}
	case v1alpha1.AddonStrategyHelmAddon:
		strategy = helmAddonStrategy{
			config: n.config.helmAddonConfig,
			client: n.client,
		}
	case "":
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("strategy not specified for cluster-autoscaler addon")
		return
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("unknown cluster-autoscaler addon deployment strategy %q", caVar.Strategy),
		)
		return
	}

	if err = strategy.delete(ctx, cluster, log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (n *DefaultClusterAutoscaler) getCAVariable(
	cluster *clusterv1.Cluster,
) (*v1alpha1.ClusterAutoscaler, error) {
	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)

	caVar, err := variables.Get[v1alpha1.ClusterAutoscaler](
		varMap,
		n.variableName,
		n.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	return &caVar, nil
}

func addonResourceNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-%s", addonName, cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey])
}
