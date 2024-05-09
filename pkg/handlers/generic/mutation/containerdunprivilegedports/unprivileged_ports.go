// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package containerdunprivilegedports

import (
	_ "embed"
	"path"

	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

const (
	// TODO Factor out this constant to a common package.
	containerdPatchesDirOnRemote = "/etc/containerd/cre.d"
)

var (
	//go:embed files/unprivileged-ports-config.toml
	unprivilegedPortsConfigDropIn             []byte
	unprivilegedPortsConfigDropInFileOnRemote = path.Join(
		containerdPatchesDirOnRemote,
		"unprivileged-ports-config.toml",
	)
)

func generateUnprivilegedPortsConfigDropIn() cabpkv1.File {
	return cabpkv1.File{
		Path:        unprivilegedPortsConfigDropInFileOnRemote,
		Content:     string(unprivilegedPortsConfigDropIn),
		Permissions: "0600",
	}
}
