// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	defaultMultusReleaseName = "multus"
	defaultMultusNamespace   = metav1.NamespaceSystem
)

type MultusConfig struct {
	*options.GlobalOptions

	helmAddonConfig *addons.HelmAddonConfig
}

func NewMultusConfig(globalOptions *options.GlobalOptions) *MultusConfig {
	return &MultusConfig{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-multus-values-template",
			defaultMultusNamespace,
			defaultMultusReleaseName,
		),
	}
}

func (m *MultusConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	m.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type MultusHandler struct {
	client              ctrlclient.Client
	config              *MultusConfig
	helmChartInfoGetter *config.HelmChartGetter
}

var (
	_ commonhandlers.Named                   = &MultusHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &MultusHandler{}
	_ lifecycle.BeforeClusterUpgrade         = &MultusHandler{}
)

func New(
	c ctrlclient.Client,
	cfg *MultusConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *MultusHandler {
	return &MultusHandler{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
	}
}

func (m *MultusHandler) Name() string {
	return "MultusHandler"
}

func (m *MultusHandler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	cluster, err := capiutils.ConvertV1Beta1ClusterToV1Beta2(&req.Cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to convert cluster: %v", err))
		return
	}
	commonResponse := &runtimehooksv1.CommonResponse{}
	m.apply(ctx, cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (m *MultusHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	cluster, err := capiutils.ConvertV1Beta1ClusterToV1Beta2(&req.Cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to convert cluster: %v", err))
		return
	}
	commonResponse := &runtimehooksv1.CommonResponse{}
	m.apply(ctx, cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (m *MultusHandler) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	// Check if Multus is supported for this cloud provider
	provider := capiutils.GetProvider(cluster)
	if provider != "eks" && provider != "nutanix" {
		log.V(5).Info(
			"Multus is not supported for this cloud provider. Skipping Multus deployment.",
		)
		return
	}

	log.Info(fmt.Sprintf("Cluster is %s. Checking CNI configuration for Multus deployment.", provider))

	// Read CNI configuration to detect which CNI is deployed
	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)

	cniVar, err := variables.Get[v1alpha1.CNI](
		varMap,
		v1alpha1.ClusterConfigVariableName,
		[]string{"addons", v1alpha1.CNIVariableName}...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("No CNI specified in cluster config. Skipping Multus deployment.")
			return
		}
		log.Error(err, "failed to read CNI configuration from cluster definition")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to read CNI configuration: %v", err))
		return
	}

	log.Info(fmt.Sprintf("Auto-deploying Multus for %s cluster with %s CNI", provider, cniVar.Provider))

	// Get helm chart configuration
	helmChart, err := m.helmChartInfoGetter.For(ctx, log, config.Multus)
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

	// Create and apply helm addon using existing addons package
	targetNamespace := m.config.DefaultsNamespace()
	strategy := addons.NewHelmAddonApplier(
		m.config.helmAddonConfig,
		m.client,
		helmChart,
	).
		WithValueTemplater(templateValuesFunc(cniVar)).
		WithDefaultWaiter()

	if err := strategy.Apply(ctx, cluster, targetNamespace, log); err != nil {
		log.Error(err, "failed to deploy Multus")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to deploy Multus: %v", err))
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	resp.SetMessage("Multus deployed successfully")
}
