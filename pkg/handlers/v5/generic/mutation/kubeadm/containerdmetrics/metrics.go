// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package containerdmetrics

import (
	_ "embed"

	bootstrapv1beta1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

var (
	//go:embed files/metrics-config.toml
	metricsConfigDropIn             []byte
	metricsConfigDropInFileOnRemote = common.ContainerdPatchPathOnRemote("metrics-config.toml")
)

func generateMetricsConfigDropIn() bootstrapv1beta1.File {
	return bootstrapv1beta1.File{
		Path:        metricsConfigDropInFileOnRemote,
		Content:     string(metricsConfigDropIn),
		Permissions: "0600",
	}
}
