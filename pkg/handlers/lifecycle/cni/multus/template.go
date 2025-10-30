// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// templateValuesFunc returns a template function that parses the Multus values template
// and replaces socket-related placeholders with the provided socket path.
func templateValuesFunc(socketPath string) func(*clusterv1.Cluster, string) (string, error) {
	return func(_ *clusterv1.Cluster, valuesTemplate string) (string, error) {
		t, err := template.New("").Parse(valuesTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to parse Multus values template: %w", err)
		}

		type input struct {
			SocketPath string
		}

		templateInput := input{
			SocketPath: socketPath,
		}

		var b bytes.Buffer
		err = t.Execute(&b, templateInput)
		if err != nil {
			return "", fmt.Errorf("failed to template Multus values: %w", err)
		}

		return b.String(), nil
	}
}
