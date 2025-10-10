// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// templateValues dynamically configures Multus values based on primary CNI provider.
func templateValues(cluster *clusterv1.Cluster, cniProvider string) (string, error) {
	var socketPath, cniType string

	switch cniProvider {
	case v1alpha1.CNIProviderCilium:
		socketPath = "/run/cilium/cilium.sock"
		cniType = "cilium-cni"
	case v1alpha1.CNIProviderCalico:
		socketPath = "/var/run/calico/cni-server.sock"
		cniType = "calico"
	default:
		return "", fmt.Errorf("unsupported CNI provider: %s", cniProvider)
	}

	// Create template values
	templateText := `
primaryCNI:
  provider: {{ .Provider }}
  socketPath: {{ .SocketPath }}
  cniConfigDir: /etc/cni/net.d

daemonConfig:
  chrootDir: /hostroot
  cniVersion: "0.3.1"
  logLevel: verbose
  logToStderr: true
  readinessIndicatorFile: {{ .SocketPath }}
  cniConfigDir: /host/etc/cni/net.d
  multusAutoconfigDir: /host/etc/cni/net.d
  multusConfigFile: auto
  socketDir: /host/run/multus/
`

	tmpl, err := template.New("multus-values").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type templateInput struct {
		Provider   string
		SocketPath string
		CNIType    string
	}

	input := templateInput{
		Provider:   cniProvider,
		SocketPath: socketPath,
		CNIType:    cniType,
	}

	var b bytes.Buffer
	if err := tmpl.Execute(&b, input); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return b.String(), nil
}
