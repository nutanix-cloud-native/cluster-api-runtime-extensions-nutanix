// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/cni"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type MultusConfig struct {
	*options.GlobalOptions
}

func NewMultusConfig(globalOptions *options.GlobalOptions) *MultusConfig {
	return &MultusConfig{
		GlobalOptions: globalOptions,
	}
}

func (m *MultusConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	// No flags needed for Multus - it's auto-deployed
}

type MultusHandler struct {
	client              ctrlclient.Client
	config              *MultusConfig
	helmChartInfoGetter *config.HelmChartGetter
	deployer            *MultusDeployer
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
		deployer:            NewMultusDeployer(c, helmChartInfoGetter),
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
	commonResponse := &runtimehooksv1.CommonResponse{}
	m.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (m *MultusHandler) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	m.apply(ctx, &req.Cluster, commonResponse)
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
	isSupported, providerName := m.deployer.isCloudProviderSupported(cluster)

	if !isSupported {
		log.V(5).Info(
			"Multus is not supported for this cloud provider. Skipping Multus deployment.",
		)
		return
	}

	log.Info(fmt.Sprintf("Cluster is %s. Checking CNI configuration for Multus deployment.", providerName))

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

	// Get readiness socket path for the CNI provider
	readinessSocketPath, err := cni.ReadinessSocketPath(cniVar.Provider)
	if err != nil {
		log.V(5).
			Info(fmt.Sprintf("Multus does not support CNI provider: %s. Skipping Multus deployment.", cniVar.Provider))
		return
	}

	log.Info(fmt.Sprintf("Auto-deploying Multus for %s cluster with %s CNI", providerName, cniVar.Provider))

	// Deploy Multus using the deployer
	targetNamespace := m.config.DefaultsNamespace()
	if err := m.deployer.Deploy(ctx, cluster, readinessSocketPath, targetNamespace, log); err != nil {
		log.Error(err, "failed to deploy Multus")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to deploy Multus: %v", err))
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	resp.SetMessage("Multus deployed successfully")
}
