// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package containerdunprivilegedports

import (
	_ "embed"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

var (
	//go:embed files/unprivileged-ports-config.toml
	unprivilegedPortsConfigDropIn             []byte
	unprivilegedPortsConfigDropInFileOnRemote = common.ContainerdPatchPathOnRemote(
		"unprivileged-ports-config.toml",
	)
)

func generateUnprivilegedPortsConfigDropIn() bootstrapv1.File {
	return bootstrapv1.File{
		Path:        unprivilegedPortsConfigDropInFileOnRemote,
		Content:     string(unprivilegedPortsConfigDropIn),
		Permissions: "0600",
	}
}
