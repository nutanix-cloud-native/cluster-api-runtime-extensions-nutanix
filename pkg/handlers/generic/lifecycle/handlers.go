// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/ccm"
	awsccm "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/ccm/aws"
	nutanixccm "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/ccm/nutanix"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/clusterautoscaler"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/cni/calico"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/cni/cilium"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi"
	awsebs "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/aws-ebs"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/localpath"
	nutanixcsi "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/nutanix-csi"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/nfd"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/servicelbgc"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/serviceloadbalancer"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/serviceloadbalancer/metallb"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type Handlers struct {
	globalOptions           *options.GlobalOptions
	calicoCNIConfig         *calico.CNIConfig
	ciliumCNIConfig         *cilium.CNIConfig
	nfdConfig               *nfd.Config
	clusterAutoscalerConfig *clusterautoscaler.Config
	ebsConfig               *awsebs.AWSEBSConfig
	nutanixCSIConfig        *nutanixcsi.NutanixCSIConfig
	awsccmConfig            *awsccm.AWSCCMConfig
	nutanixCCMConfig        *nutanixccm.Config
	metalLBConfig           *metallb.Config
	localPathCSIConfig      *localpath.Config
}

func New(
	globalOptions *options.GlobalOptions,
) *Handlers {
	return &Handlers{
		globalOptions: globalOptions,
		calicoCNIConfig: &calico.CNIConfig{
			GlobalOptions: globalOptions,
		},
		ciliumCNIConfig:         &cilium.CNIConfig{GlobalOptions: globalOptions},
		nfdConfig:               &nfd.Config{GlobalOptions: globalOptions},
		clusterAutoscalerConfig: &clusterautoscaler.Config{GlobalOptions: globalOptions},
		ebsConfig:               &awsebs.AWSEBSConfig{GlobalOptions: globalOptions},
		awsccmConfig:            &awsccm.AWSCCMConfig{GlobalOptions: globalOptions},
		nutanixCSIConfig:        &nutanixcsi.NutanixCSIConfig{GlobalOptions: globalOptions},
		nutanixCCMConfig:        &nutanixccm.Config{GlobalOptions: globalOptions},
		metalLBConfig:           &metallb.Config{GlobalOptions: globalOptions},
		localPathCSIConfig:      &localpath.Config{GlobalOptions: globalOptions},
	}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	helmChartInfoGetter := config.NewHelmChartGetterFromConfigMap(
		h.globalOptions.HelmAddonsConfigMapName(),
		h.globalOptions.DefaultsNamespace(),
		mgr.GetClient(),
	)
	csiHandlers := map[string]csi.CSIProvider{
		v1alpha1.CSIProviderAWSEBS: awsebs.New(mgr.GetClient(), h.ebsConfig),
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
		v1alpha1.CCMProviderAWS: awsccm.New(mgr.GetClient(), h.awsccmConfig),
		v1alpha1.CCMProviderNutanix: nutanixccm.New(
			mgr.GetClient(),
			h.nutanixCCMConfig,
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
	return []handlers.Named{
		calico.New(mgr.GetClient(), h.calicoCNIConfig, helmChartInfoGetter),
		cilium.New(mgr.GetClient(), h.ciliumCNIConfig, helmChartInfoGetter),
		nfd.New(mgr.GetClient(), h.nfdConfig, helmChartInfoGetter),
		clusterautoscaler.New(mgr.GetClient(), h.clusterAutoscalerConfig, helmChartInfoGetter),
		servicelbgc.New(mgr.GetClient()),
		csi.New(mgr.GetClient(), csiHandlers),
		ccm.New(mgr.GetClient(), ccmHandlers),
		serviceloadbalancer.New(mgr.GetClient(), serviceLoadBalancerHandlers),
	}
}

func (h *Handlers) AddFlags(flagSet *pflag.FlagSet) {
	h.nfdConfig.AddFlags("nfd", flagSet)
	h.clusterAutoscalerConfig.AddFlags("cluster-autoscaler", flagSet)
	h.calicoCNIConfig.AddFlags("cni.calico", flagSet)
	h.ciliumCNIConfig.AddFlags("cni.cilium", flagSet)
	h.ebsConfig.AddFlags("awsebs", pflag.CommandLine)
	h.awsccmConfig.AddFlags("awsccm", pflag.CommandLine)
	h.nutanixCSIConfig.AddFlags("nutanixcsi", flagSet)
	h.nutanixCCMConfig.AddFlags("nutanixccm", flagSet)
	h.metalLBConfig.AddFlags("metallb", flagSet)
	h.localPathCSIConfig.AddFlags("csi.local-path", flagSet)
}
