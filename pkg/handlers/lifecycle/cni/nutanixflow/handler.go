// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixflow

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultNutanixFlowReleaseName = "flow-cni"
	defaultNutanixFlowNamespace   = "kube-system"
)

type CNIConfig struct {
	*options.GlobalOptions

	helmAddonConfig helmAddonConfig
}

type helmAddonConfig struct {
	defaultValuesTemplateConfigMapName string
}

func (c *helmAddonConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultValuesTemplateConfigMapName,
		prefix+".default-values-template-configmap-name",
		"default-nutanix-flow-cni-helm-values-template",
		"default values ConfigMap name",
	)
}

func (c *CNIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type NutanixFlowCNI struct {
	client              ctrlclient.Client
	config              *CNIConfig
	helmChartInfoGetter *config.HelmChartGetter

	variableName string
	variablePath []string
}

var (
	_ commonhandlers.Named                   = &NutanixFlowCNI{}
	_ lifecycle.AfterControlPlaneInitialized = &NutanixFlowCNI{}
	_ lifecycle.BeforeClusterUpgrade         = &NutanixFlowCNI{}
)

func New(
	c ctrlclient.Client,
	cfg *CNIConfig,
	helmChartInfoGetter *config.HelmChartGetter,
) *NutanixFlowCNI {
	return &NutanixFlowCNI{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.CNIVariableName},
	}
}

func (c *NutanixFlowCNI) Name() string {
	return "NutanixFlowCNI"
}

func (c *NutanixFlowCNI) AfterControlPlaneInitialized(
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
	c.apply(ctx, cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *NutanixFlowCNI) BeforeClusterUpgrade(
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
	c.apply(ctx, cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (c *NutanixFlowCNI) apply(
	ctx context.Context,
	cluster *clusterv1beta2.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	if cluster.Spec.InfrastructureRef.Kind != "NutanixCluster" {
		log.V(5).Info(
			"Skipping Nutanix Flow CNI handler, cluster infrastructure is not NutanixCluster",
		)
		return
	}

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)

	cniVar, err := variables.Get[v1alpha1.GenericCNI](varMap, c.variableName, c.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info(
				"Skipping Nutanix Flow CNI handler, cluster does not specify CNI addon deployment",
			)
			return
		}
		log.Error(err, "failed to read CNI provider from cluster definition")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CNI provider from cluster definition: %v", err),
		)
		return
	}
	if cniVar.Provider != v1alpha1.CNIProviderFlow {
		log.V(5).Info(
			fmt.Sprintf(
				"Skipping Nutanix Flow CNI handler, cluster does not specify %q as value of CNI provider variable",
				v1alpha1.CNIProviderFlow,
			),
		)
		return
	}

	if cniVar.Strategy != v1alpha1.AddonStrategyHelmAddon {
		log.Error(nil, fmt.Sprintf(
			"Nutanix Flow CNI only supports %q strategy, got %q",
			v1alpha1.AddonStrategyHelmAddon,
			cniVar.Strategy,
		))
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf(
			"Nutanix Flow CNI only supports %q addon strategy",
			v1alpha1.AddonStrategyHelmAddon,
		))
		return
	}

	helmChart, err := c.helmChartInfoGetter.For(ctx, log, config.NutanixFlowCNI)
	if err != nil {
		log.Error(err, "failed to get configmap with helm settings")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to get configuration to create helm addon: %v", err),
		)
		return
	}

	targetNamespace := c.config.DefaultsNamespace()

	helmValuesSourceRefName := c.config.helmAddonConfig.defaultValuesTemplateConfigMapName
	if cniVar.Values != nil && cniVar.Values.SourceRef != nil {
		helmValuesSourceRefName = cniVar.Values.SourceRef.Name
		targetNamespace = cluster.Namespace

		err := handlersutils.EnsureClusterOwnerReferenceForObject(
			ctx,
			c.client,
			corev1.TypedLocalObjectReference{
				Kind: cniVar.Values.SourceRef.Kind,
				Name: cniVar.Values.SourceRef.Name,
			},
			cluster,
		)
		if err != nil {
			log.Error(
				err,
				"error updating Cluster's owner reference on Flow CNI helm values source object",
				"name", cniVar.Values.SourceRef.Name,
				"kind", cniVar.Values.SourceRef.Kind,
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(fmt.Sprintf(
				"failed to set Cluster's owner reference on Flow CNI helm values source object: %v",
				err,
			))
			return
		}
	}

	strategy := addons.NewHelmAddonApplier(
		addons.NewHelmAddonConfig(
			helmValuesSourceRefName,
			defaultNutanixFlowNamespace,
			defaultNutanixFlowReleaseName,
		),
		c.client,
		helmChart,
	).
		WithValueTemplater(templateValues).
		WithDefaultWaiter()

	if err := strategy.Apply(ctx, cluster, targetNamespace, log); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
