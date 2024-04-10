// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/ccm"
	awsccm "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/ccm/aws"
	nutanixccm "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/ccm/nutanix"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/clusterautoscaler"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/cni/calico"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/cni/cilium"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/config"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi"
	awsebs "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/aws-ebs"
	nutanixcsi "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/nutanix-csi"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/nfd"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/servicelbgc"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type Handlers struct {
	globalOptions           *options.GlobalOptions
	calicoCNIConfig         *calico.CNIConfig
	ciliumCNIConfig         *cilium.CNIConfig
	nfdConfig               *nfd.Config
	clusterAutoscalerConfig *clusterautoscaler.Config
	ebsConfig               *awsebs.AWSEBSConfig
	nutnaixCSIConfig        *nutanixcsi.NutanixCSIConfig
	awsccmConfig            *awsccm.AWSCCMConfig
	nutanixCCMConfig        *nutanixccm.Config
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
		nutnaixCSIConfig:        &nutanixcsi.NutanixCSIConfig{GlobalOptions: globalOptions},
		nutanixCCMConfig:        &nutanixccm.Config{GlobalOptions: globalOptions},
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
			h.nutnaixCSIConfig,
			helmChartInfoGetter,
		),
	}
	ccmHandlers := map[string]ccm.CCMProvider{
		v1alpha1.CCMProviderAWS:     awsccm.New(mgr.GetClient(), h.awsccmConfig),
		v1alpha1.CCMProviderNutanix: nutanixccm.New(mgr.GetClient(), h.nutanixCCMConfig, helmChartInfoGetter),
	}
	return []handlers.Named{
		calico.New(mgr.GetClient(), h.calicoCNIConfig, helmChartInfoGetter),
		cilium.New(mgr.GetClient(), h.ciliumCNIConfig, helmChartInfoGetter),
		nfd.New(mgr.GetClient(), h.nfdConfig, helmChartInfoGetter),
		clusterautoscaler.New(mgr.GetClient(), h.clusterAutoscalerConfig, helmChartInfoGetter),
		servicelbgc.New(mgr.GetClient()),
		csi.New(mgr.GetClient(), csiHandlers),
		ccm.New(mgr.GetClient(), ccmHandlers),
	}
}

func (h *Handlers) AddFlags(flagSet *pflag.FlagSet) {
	h.nfdConfig.AddFlags("nfd", flagSet)
	h.clusterAutoscalerConfig.AddFlags("cluster-autoscaler", flagSet)
	h.calicoCNIConfig.AddFlags("cni.calico", flagSet)
	h.ciliumCNIConfig.AddFlags("cni.cilium", flagSet)
	h.ebsConfig.AddFlags("awsebs", pflag.CommandLine)
	h.awsccmConfig.AddFlags("awsccm", pflag.CommandLine)
	h.nutnaixCSIConfig.AddFlags("nutanixcsi", flagSet)
	h.nutanixCCMConfig.AddFlags("nutanixccm", flagSet)
}
