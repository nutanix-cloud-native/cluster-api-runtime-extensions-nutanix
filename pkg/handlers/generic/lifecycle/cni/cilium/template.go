// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// templateValues enables kube-proxy replacement when kube-proxy is disabled.
func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	ciliumTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		EnableKubeProxyReplacement bool
	}

	// If kube-proxy is not disabled, return early.
	// Otherwise, wait for Cilium to be rolled out and then cleanup kube-proxy if installed.
	isKuberProxyDisabled, err := isKubeProxyDisabled(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to check if kube-proxy is disabled: %w", err)
	}

	// Assume when kube-proxy is skipped, we should enable Cilium's kube-proxy replacement feature.
	templateInput := input{
		EnableKubeProxyReplacement: isKuberProxyDisabled,
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
