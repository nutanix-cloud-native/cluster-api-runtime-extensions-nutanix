// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

// templateValues enables kube-proxy replacement when kube-proxy is disabled.
func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	kubeProxyIsDisabled, err := apivariables.KubeProxyIsDisabled(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to check if kube-proxy is disabled: %w", err)
	}

	ciliumTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		Cluster                    *clusterv1.Cluster
		EnableKubeProxyReplacement bool
	}

	// Assume when kube-proxy is disabled, we should enable Cilium's kube-proxy replacement feature.
	templateInput := input{
		EnableKubeProxyReplacement: kubeProxyIsDisabled,
	}

	var b bytes.Buffer
	err = ciliumTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf(
			"failed templating Cilium values: %w",
			err,
		)
	}

	return b.String(), nil
}
