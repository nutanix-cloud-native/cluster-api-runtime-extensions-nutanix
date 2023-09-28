// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/cni/calico"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/nfd"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/servicelbgc"
)

type Handlers struct {
	calicoCNIConfig *calico.CalicoCNIConfig
	nfdConfig       *nfd.NFDConfig
}

func New() *Handlers {
	calicoCNIConfig := &calico.CalicoCNIConfig{}
	nfdConfig := &nfd.NFDConfig{}

	return &Handlers{
		calicoCNIConfig: calicoCNIConfig,
		nfdConfig:       nfdConfig,
	}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	return []handlers.Named{
		calico.New(mgr.GetClient(), h.calicoCNIConfig),
		nfd.New(mgr.GetClient(), h.nfdConfig),
		servicelbgc.New(mgr.GetClient()),
	}
}

func (h *Handlers) AddFlags(flagSet *pflag.FlagSet) {
	h.nfdConfig.AddFlags("nfd", flagSet)
	h.calicoCNIConfig.AddFlags("calicocni", flagSet)
}
