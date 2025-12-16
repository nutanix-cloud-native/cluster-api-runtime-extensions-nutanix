// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/cni"
)

// templateValuesFunc returns a template function that parses the Multus values template
// and replaces socket-related placeholders with the readiness socket path.
// It looks up the socket path internally based on the CNI provider.
func templateValuesFunc(cniVar v1alpha1.CNI) func(*clusterv1.Cluster, string) (string, error) {
	return func(_ *clusterv1.Cluster, valuesTemplate string) (string, error) {
		// Look up the readiness socket path for the CNI provider
		readinessSocketPath, err := cni.ReadinessSocketPath(cniVar.Provider)
		if err != nil {
			return "", fmt.Errorf("failed to get readiness socket path for CNI provider %s: %w", cniVar.Provider, err)
		}

		t, err := template.New("").Parse(valuesTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to parse Multus values template: %w", err)
		}

		type input struct {
			ReadinessSocketPath string
		}

		templateInput := input{
			ReadinessSocketPath: readinessSocketPath,
		}

		var b bytes.Buffer
		err = t.Execute(&b, templateInput)
		if err != nil {
			return "", fmt.Errorf("failed to template Multus values: %w", err)
		}

		return b.String(), nil
	}
}
