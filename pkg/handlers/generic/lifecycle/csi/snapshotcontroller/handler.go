// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package snapshotcontroller

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	defaultHelmReleaseName      = "snapshot-controller"
	defaultHelmReleaseNamespace = metav1.NamespaceSystem
)

type Config struct {
	*options.GlobalOptions

	crsConfig       crsConfig
	helmAddonConfig *addons.HelmAddonConfig
}

func NewConfig(globalOptions *options.GlobalOptions) *Config {
	return &Config{
		GlobalOptions: globalOptions,
		crsConfig:     crsConfig{},
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-snapshot-controller-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.crsConfig.AddFlags(prefix+".crs", flags)
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

var (
	_ commonhandlers.Named                   = &SnapshotControllerHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &SnapshotControllerHandler{}
	_ lifecycle.BeforeClusterUpgrade         = &SnapshotControllerHandler{}
)

type SnapshotControllerHandler struct {
	client              ctrlclient.Client
	variableName        string
	variablePath        []string
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter
}

func (s *SnapshotControllerHandler) Name() string {
	return "SnapshotControllerHandler"
}

func (s *SnapshotControllerHandler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	s.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (s *SnapshotControllerHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	s.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *SnapshotControllerHandler {
	return &SnapshotControllerHandler{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", "csi", "snapshotController"},
	}
}

func (s *SnapshotControllerHandler) apply(
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
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	snapshotControllerVar, err := variables.Get[v1alpha1.SnapshotController](
		varMap,
		s.variableName,
		s.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info("Skipping snapshot-controller handler, the cluster does not define the snapshot-controller variable")
			return
		}
		msg := "failed to read the snapshot-controller variable from the cluster"
		log.Error(err, msg)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("%s: %v", msg, err))
		return
	}

	var strategy addons.Applier
	switch snapshotControllerVar.Strategy {
	case v1alpha1.AddonStrategyHelmAddon:
		helmChart, err := s.helmChartInfoGetter.For(ctx, log, config.SnapshotController)
		if err != nil {
			msg := "failed to get configuration to create helm addon"
			log.Error(err, msg)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(fmt.Sprintf("%s: %v", msg, err))
			return
		}
		strategy = addons.NewHelmAddonApplier(
			s.config.helmAddonConfig,
			s.client,
			helmChart,
		)
	case v1alpha1.AddonStrategyClusterResourceSet:
		strategy = crsStrategy{
			config: s.config.crsConfig,
			client: s.client,
		}
	case "":
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("strategy not provided for snapshot-controller")
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"unknown snapshot-controller addon deployment strategy %q",
				snapshotControllerVar.Strategy,
			),
		)
	}

	if err := strategy.Apply(ctx, cluster, s.config.DefaultsNamespace(), log); err != nil {
		err = fmt.Errorf("failed to apply snapshot-controller addon: %w", err)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
