// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/generic/kubeproxymode"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/generic/ntp"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/generic/taints"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/generic/users"
)

// metaMutators returns all EKS applicable patch handlers.
func metaMutators() []mutation.MetaMutator {
	return []mutation.MetaMutator{
		users.NewPatch(),
		kubeproxymode.NewPatch(),
		ntp.NewPatch(),
	}
}

func workerMetaMutators() []mutation.MetaMutator {
	return []mutation.MetaMutator{
		taints.NewWorkerPatch(),
	}
}
