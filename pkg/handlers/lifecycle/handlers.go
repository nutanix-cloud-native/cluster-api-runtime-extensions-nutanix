// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"github.com/samber/lo"
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/ccm"
	awsccm "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/ccm/aws"
	nutanixccm "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/ccm/nutanix"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/clusterautoscaler"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/cni/calico"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/cni/cilium"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/cosi"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/csi"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/csi/awsebs"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/csi/localpath"
	nutanixcsi "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/csi/nutanix"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/csi/snapshotcontroller"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/ingress/awsloadbalancercontroller"
	konnectoragent "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/konnectoragent"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/nfd"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/registry"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/registry/cncfdistribution"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/servicelbgc"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/serviceloadbalancer"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/serviceloadbalancer/metallb"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type Handlers struct {
	globalOptions                   *options.GlobalOptions
	calicoCNIConfig                 *calico.CNIConfig
	ciliumCNIConfig                 *cilium.CNIConfig
	nfdConfig                       *nfd.Config
	clusterAutoscalerConfig         *clusterautoscaler.Config
	ebsConfig                       *awsebs.Config
	nutanixCSIConfig                *nutanixcsi.Config
	awsccmConfig                    *awsccm.AWSCCMConfig
	nutanixCCMConfig                *nutanixccm.Config
	metalLBConfig                   *metallb.Config
	localPathCSIConfig              *localpath.Config
	snapshotControllerConfig        *snapshotcontroller.Config
	cosiControllerConfig            *cosi.ControllerConfig
	awsLoadBalancerControllerConfig *awsloadbalancercontroller.ControllerConfig
	konnectorAgentConfig            *konnectoragent.Config
	distributionConfig              *cncfdistribution.Config
}

func New(
	globalOptions *options.GlobalOptions,
) *Handlers {
	return &Handlers{
		globalOptions: globalOptions,
		calicoCNIConfig: &calico.CNIConfig{
			GlobalOptions: globalOptions,
		},
		ciliumCNIConfig:                 &cilium.CNIConfig{GlobalOptions: globalOptions},
		nfdConfig:                       nfd.NewConfig(globalOptions),
		clusterAutoscalerConfig:         &clusterautoscaler.Config{GlobalOptions: globalOptions},
		ebsConfig:                       awsebs.NewConfig(globalOptions),
		awsccmConfig:                    awsccm.NewConfig(globalOptions),
		awsLoadBalancerControllerConfig: awsloadbalancercontroller.NewControllerConfig(globalOptions),
		nutanixCSIConfig:                nutanixcsi.NewConfig(globalOptions),
		nutanixCCMConfig:                &nutanixccm.Config{GlobalOptions: globalOptions},
		metalLBConfig:                   &metallb.Config{GlobalOptions: globalOptions},
		localPathCSIConfig:              localpath.NewConfig(globalOptions),
		snapshotControllerConfig:        snapshotcontroller.NewConfig(globalOptions),
		cosiControllerConfig:            cosi.NewControllerConfig(globalOptions),
		konnectorAgentConfig:            konnectoragent.NewConfig(globalOptions),
		distributionConfig:              &cncfdistribution.Config{GlobalOptions: globalOptions},
	}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	helmChartInfoGetter := config.NewHelmChartGetterFromConfigMap(
		h.globalOptions.HelmAddonsConfigMapName(),
		h.globalOptions.DefaultsNamespace(),
		mgr.GetClient(),
	)
	csiHandlers := map[string]csi.CSIProvider{
		v1alpha1.CSIProviderAWSEBS: awsebs.New(
			mgr.GetClient(),
			h.ebsConfig,
			helmChartInfoGetter,
		),
		v1alpha1.CSIProviderNutanix: nutanixcsi.New(
			mgr.GetClient(),
			h.nutanixCSIConfig,
			helmChartInfoGetter,
		),
		v1alpha1.CSIProviderLocalPath: localpath.New(
			mgr.GetClient(),
			h.localPathCSIConfig,
			helmChartInfoGetter,
		),
	}
	ccmHandlers := map[string]ccm.CCMProvider{
		v1alpha1.CCMProviderAWS: awsccm.New(mgr.GetClient(), h.awsccmConfig, helmChartInfoGetter),
		v1alpha1.CCMProviderNutanix: nutanixccm.New(
			mgr.GetClient(),
			h.nutanixCCMConfig,
			helmChartInfoGetter,
		),
	}
	registryHandlers := map[string]registry.RegistryProvider{
		v1alpha1.RegistryProviderCNCFDistribution: cncfdistribution.New(
			mgr.GetClient(),
			h.distributionConfig,
			helmChartInfoGetter,
		),
	}
	serviceLoadBalancerHandlers := map[string]serviceloadbalancer.ServiceLoadBalancerProvider{
		v1alpha1.ServiceLoadBalancerProviderMetalLB: metallb.New(
			mgr.GetClient(),
			h.metalLBConfig,
			helmChartInfoGetter,
		),
	}
	allHandlers := []handlers.Named{
		calico.New(mgr.GetClient(), h.calicoCNIConfig, helmChartInfoGetter),
		cilium.New(mgr.GetClient(), h.ciliumCNIConfig, helmChartInfoGetter),
		ccm.New(mgr.GetClient(), ccmHandlers),
		nfd.New(mgr.GetClient(), h.nfdConfig, helmChartInfoGetter),
		clusterautoscaler.New(mgr.GetClient(), h.clusterAutoscalerConfig, helmChartInfoGetter),
		csi.New(mgr.GetClient(), csiHandlers),
		snapshotcontroller.New(mgr.GetClient(), h.snapshotControllerConfig, helmChartInfoGetter),
		cosi.New(mgr.GetClient(), h.cosiControllerConfig, helmChartInfoGetter),
		konnectoragent.New(mgr.GetClient(), h.konnectorAgentConfig, helmChartInfoGetter),
		awsloadbalancercontroller.New(mgr.GetClient(), h.awsLoadBalancerControllerConfig, helmChartInfoGetter),
		servicelbgc.New(mgr.GetClient()),
		registry.New(mgr.GetClient(), registryHandlers),
		// The order of the handlers in the list is important and are called consecutively.
		// The MetalLB provider may be configured to create a IPAddressPool on the remote cluster.
		// However, the MetalLB provider also has a webhook that validates IPAddressPool requests.
		// Because this webhook relies on CNI and CCM to already be installed on the remote cluster,
		// we are placing this handler after the CNI and CCM handlers.
		serviceloadbalancer.New(mgr.GetClient(), serviceLoadBalancerHandlers),
	}

	var orderedHandlers []handlers.Named

	bccHandlers := lo.FlatMap(
		allHandlers,
		func(h handlers.Named, _ int) []lifecycle.BeforeClusterCreate {
			if h, ok := h.(lifecycle.BeforeClusterCreate); ok {
				return []lifecycle.BeforeClusterCreate{h}
			}
			return nil
		})
	if len(bccHandlers) > 0 {
		orderedHandlers = append(
			orderedHandlers,
			lifecycle.ParallelBeforeClusterCreateHook("caren", bccHandlers...),
		)
	}

	acpiHandlers := lo.FlatMap(
		allHandlers,
		func(h handlers.Named, _ int) []lifecycle.AfterControlPlaneInitialized {
			if h, ok := h.(lifecycle.AfterControlPlaneInitialized); ok {
				return []lifecycle.AfterControlPlaneInitialized{h}
			}
			return nil
		})
	if len(acpiHandlers) > 0 {
		orderedHandlers = append(
			orderedHandlers,
			lifecycle.ParallelAfterControlPlaneInitializedHook("caren", acpiHandlers...),
		)
	}

	bcuHandlers := lo.FlatMap(
		allHandlers,
		func(h handlers.Named, _ int) []lifecycle.BeforeClusterUpgrade {
			if h, ok := h.(lifecycle.BeforeClusterUpgrade); ok {
				return []lifecycle.BeforeClusterUpgrade{h}
			}
			return nil
		})
	if len(bcuHandlers) > 0 {
		orderedHandlers = append(
			orderedHandlers,
			lifecycle.ParallelBeforeClusterUpgradeHook("caren", bcuHandlers...),
		)
	}

	acpuHandlers := lo.FlatMap(
		allHandlers,
		func(h handlers.Named, _ int) []lifecycle.AfterControlPlaneUpgrade {
			if h, ok := h.(lifecycle.AfterControlPlaneUpgrade); ok {
				return []lifecycle.AfterControlPlaneUpgrade{h}
			}
			return nil
		})
	if len(acpuHandlers) > 0 {
		orderedHandlers = append(
			orderedHandlers,
			lifecycle.ParallelAfterControlPlaneUpgradeHook("caren", acpuHandlers...),
		)
	}

	bcdHandlers := lo.FlatMap(
		allHandlers,
		func(h handlers.Named, _ int) []lifecycle.BeforeClusterDelete {
			if h, ok := h.(lifecycle.BeforeClusterDelete); ok {
				return []lifecycle.BeforeClusterDelete{h}
			}
			return nil
		})
	if len(bcdHandlers) > 0 {
		orderedHandlers = append(
			orderedHandlers,
			lifecycle.ParallelBeforeClusterDeleteHook("caren", bcdHandlers...),
		)
	}

	return orderedHandlers
}

func (h *Handlers) AddFlags(flagSet *pflag.FlagSet) {
	h.nfdConfig.AddFlags("nfd", flagSet)
	h.clusterAutoscalerConfig.AddFlags("cluster-autoscaler", flagSet)
	h.calicoCNIConfig.AddFlags("cni.calico", flagSet)
	h.ciliumCNIConfig.AddFlags("cni.cilium", flagSet)
	h.ebsConfig.AddFlags("csi.aws-ebs", pflag.CommandLine)
	h.nutanixCSIConfig.AddFlags("csi.nutanix", flagSet)
	h.localPathCSIConfig.AddFlags("csi.local-path", flagSet)
	h.snapshotControllerConfig.AddFlags("csi.snapshot-controller", flagSet)
	h.awsccmConfig.AddFlags("ccm.aws", pflag.CommandLine)
	h.nutanixCCMConfig.AddFlags("ccm.nutanix", flagSet)
	h.metalLBConfig.AddFlags("metallb", flagSet)
	h.cosiControllerConfig.AddFlags("cosi.controller", flagSet)
	h.konnectorAgentConfig.AddFlags("konnector-agent", flagSet)
	h.distributionConfig.AddFlags("registry.cncf-distribution", flagSet)
}
