// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/auditpolicy"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/etcd"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/extraapiservercertsans"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/httpproxy"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries/credentials"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/kubernetesimagerepository"
)

// MetaMutators returns all generic patch handlers.
func MetaMutators(mgr manager.Manager) []mutation.MetaMutator {
	return []mutation.MetaMutator{
		auditpolicy.NewPatch(),
		etcd.NewMetaPatch(),
		extraapiservercertsans.NewMetaPatch(),
		httpproxy.NewMetaPatch(mgr.GetClient()),
		kubernetesimagerepository.NewMetaPatch(),
		credentials.NewMetaPatch(mgr.GetClient()),
	}
}
