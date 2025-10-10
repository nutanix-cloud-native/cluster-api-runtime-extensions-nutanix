// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// templateValuesFunc returns a template function that parses the Multus values template
// and replaces socket-related placeholders with the provided readiness socket path.
func templateValuesFunc(readinessSocketPath string) func(*clusterv1.Cluster, string) (string, error) {
	return func(_ *clusterv1.Cluster, valuesTemplate string) (string, error) {
		t, err := template.New("").Parse(valuesTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to parse Multus values template: %w", err)
		}

		type input struct {
			ReadinessSocketPath string
			SocketVolumeName    string
		}

		// Extract volume name from socket path
		// e.g., "/run/cilium/cilium.sock" -> "cilium-sock"
		// or "/var/run/calico/cni-server.sock" -> "calico-sock"
		socketVolumeName := extractVolumeName(readinessSocketPath)

		templateInput := input{
			ReadinessSocketPath: readinessSocketPath,
			SocketVolumeName:    socketVolumeName,
		}

		var b bytes.Buffer
		err = t.Execute(&b, templateInput)
		if err != nil {
			return "", fmt.Errorf("failed to template Multus values: %w", err)
		}

		return b.String(), nil
	}
}

// extractVolumeName derives a volume name from the socket path.
// Examples:
//   - "/run/cilium/cilium.sock" -> "cilium-sock"
//   - "/var/run/calico/cni-server.sock" -> "calico-sock"
func extractVolumeName(socketPath string) string {
	if socketPath == "" {
		return ""
	}

	// Get the directory part and extract the CNI name
	dir := filepath.Dir(socketPath)
	// Remove leading/trailing slashes and get the last component
	dir = strings.Trim(dir, "/")
	parts := strings.Split(dir, "/")

	var name string
	// Look for "cilium" or "calico" in the path
	for _, part := range parts {
		if part == "cilium" || part == "calico" {
			name = part
			break
		}
	}

	if name == "" {
		// Fallback: use the last directory component before the socket
		if len(parts) > 0 {
			name = parts[len(parts)-1]
		}
	}

	return name + "-sock"
}
