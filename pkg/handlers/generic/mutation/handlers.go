// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/cni/calico"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/auditpolicy"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/etcd"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/extraapiservercertsans"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/httpproxy"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/imageregistries/credentials"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubernetesimagerepository"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/mirrors"
)

// MetaMutators returns all generic patch handlers.
func MetaMutators(mgr manager.Manager) []mutation.MetaMutator {
	return []mutation.MetaMutator{
		auditpolicy.NewPatch(),
		etcd.NewPatch(),
		extraapiservercertsans.NewPatch(mgr.GetClient()),
		httpproxy.NewPatch(mgr.GetClient()),
		kubernetesimagerepository.NewPatch(),
		credentials.NewPatch(mgr.GetClient()),
		mirrors.NewPatch(mgr.GetClient()),
		calico.NewPatch(),
	}
}
