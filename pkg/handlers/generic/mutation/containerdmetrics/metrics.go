// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package containerdmetrics

import (
	_ "embed"
	"path"

	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

const (
	// TODO Factor out this constant to a common package.
	containerdPatchesDirOnRemote = "/etc/caren/containerd"
)

var (
	//go:embed files/metrics-config.toml
	metricsConfigDropIn             []byte
	metricsConfigDropInFileOnRemote = path.Join(
		containerdPatchesDirOnRemote,
		"metrics-config.toml",
	)
)

func generateMetricsConfigDropIn() cabpkv1.File {
	return cabpkv1.File{
		Path:        metricsConfigDropInFileOnRemote,
		Content:     string(metricsConfigDropIn),
		Permissions: "0600",
	}
}
