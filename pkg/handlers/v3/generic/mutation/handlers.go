// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/cni/calico"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/autorenewcerts"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/containerdapplypatchesandrestart"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/containerdmetrics"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/containerdunprivilegedports"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/coredns"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/encryptionatrest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/etcd"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/extraapiservercertsans"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/httpproxy"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/imageregistries/credentials"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubernetesimagerepository"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/mirrors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/taints"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/users"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v3/generic/mutation/auditpolicy"
)

// MetaMutators returns all generic patch handlers.
func MetaMutators(mgr manager.Manager) []mutation.MetaMutator {
	return []mutation.MetaMutator{
		auditpolicy.NewPatch(),
		etcd.NewPatch(),
		coredns.NewPatch(),
		extraapiservercertsans.NewPatch(),
		httpproxy.NewPatch(mgr.GetClient()),
		kubernetesimagerepository.NewPatch(),
		credentials.NewPatch(mgr.GetClient()),
		mirrors.NewPatch(mgr.GetClient()),
		calico.NewPatch(),
		users.NewPatch(),
		containerdmetrics.NewPatch(),
		containerdunprivilegedports.NewPatch(),
		encryptionatrest.NewPatch(mgr.GetClient(), encryptionatrest.RandomTokenGenerator),
		autorenewcerts.NewPatch(),

		// Some patches may have changed containerd configuration.
		// We write the configuration changes to disk, and must run a command
		// to apply the changes to the actual containerd configuration.
		// We then must restart containerd for the configuration to take effect.
		// Therefore, we must apply this patch last.
		//
		// Containerd restart and readiness altogether could take ~5s.
		// We want to keep patch independent of each other and not share any state.
		// Therefore, We must always apply this patch regardless any other patch modified containerd configuration.
		containerdapplypatchesandrestart.NewPatch(),
	}
}

func ControlPlaneMetaMutators() []mutation.MetaMutator {
	return []mutation.MetaMutator{
		taints.NewControlPlanePatch(),
	}
}

func WorkerMetaMutators() []mutation.MetaMutator {
	return []mutation.MetaMutator{
		taints.NewWorkerPatch(),
	}
}
