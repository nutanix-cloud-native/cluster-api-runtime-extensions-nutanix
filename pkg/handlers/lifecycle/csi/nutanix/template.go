// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	nutanixCSITemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		BareMetalMode bool
	}

	templateInput := input{
		BareMetalMode: cluster.Annotations[v1alpha1.ExperimentalNutanixCSIBaremetalMode] == "true",
	}

	var b bytes.Buffer
	if err = nutanixCSITemplate.Execute(&b, templateInput); err != nil {
		return "", fmt.Errorf("failed templating Nutanix CSI values: %w", err)
	}

	return b.String(), nil
}
