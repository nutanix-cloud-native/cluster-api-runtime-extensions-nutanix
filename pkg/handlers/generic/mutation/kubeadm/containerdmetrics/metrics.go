// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package containerdmetrics

import (
	_ "embed"

	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

var (
	//go:embed files/metrics-config.toml
	metricsConfigDropIn             []byte
	metricsConfigDropInFileOnRemote = common.ContainerdPatchPathOnRemote("metrics-config.toml")
)

func generateMetricsConfigDropIn() cabpkv1.File {
	return cabpkv1.File{
		Path:        metricsConfigDropInFileOnRemote,
		Content:     string(metricsConfigDropIn),
		Permissions: "0600",
	}
}
