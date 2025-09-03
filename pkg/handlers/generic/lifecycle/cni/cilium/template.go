// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

// templateValues enables kube-proxy replacement when skip kube-proxy annotation is set.
func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	ciliumTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		EnableKubeProxyReplacement bool
	}

	// Assume when kube-proxy is skipped, we should enable Cilium's kube-proxy replacement feature.
	templateInput := input{
		EnableKubeProxyReplacement: capiutils.SkipKubeProxy(cluster),
	}

	var b bytes.Buffer
	err = ciliumTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf(
			"failed setting target Cluster name and namespace in template: %w",
			err,
		)
	}

	return b.String(), nil
}
