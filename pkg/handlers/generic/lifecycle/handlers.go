// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/cni/calico"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/csi"
	awsebs "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/csi/aws-ebs"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/nfd"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/servicelbgc"
)

type Handlers struct {
	calicoCNIConfig *calico.CalicoCNIConfig
	nfdConfig       *nfd.NFDConfig
	ebsConfig       *awsebs.AWSEBSConfig
}

func New() *Handlers {
	calicoCNIConfig := &calico.CalicoCNIConfig{}
	nfdConfig := &nfd.NFDConfig{}
	ebsConfig := &awsebs.AWSEBSConfig{}

	return &Handlers{
		calicoCNIConfig: calicoCNIConfig,
		nfdConfig:       nfdConfig,
		ebsConfig:       ebsConfig,
	}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	csiHandlers := map[string]csi.CSIProvider{
		v1alpha1.CSIProviderAWSEBS: awsebs.New(mgr.GetClient(), h.ebsConfig),
	}
	return []handlers.Named{
		calico.New(mgr.GetClient(), h.calicoCNIConfig),
		nfd.New(mgr.GetClient(), h.nfdConfig),
		servicelbgc.New(mgr.GetClient()),
		csi.New(mgr.GetClient(), csiHandlers),
	}
}

func (h *Handlers) AddFlags(flagSet *pflag.FlagSet) {
	h.nfdConfig.AddFlags("nfd", flagSet)
	h.calicoCNIConfig.AddFlags("calicocni", flagSet)
	h.ebsConfig.AddFlags("awsebs", pflag.CommandLine)
}
