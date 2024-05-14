// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	nameTemplate      = "tmpl-clustername-tmpl"
	namespaceTemplate = "tmpl-clusternamespace-tmpl"
)

// templateData replaces 'tmpl-clustername-tmpl' and 'tmpl-clusternamespace-tmpl' in data.
func templateData(cluster *clusterv1.Cluster, data map[string]string) map[string]string {
	templated := make(map[string]string, len(data))
	for k, v := range data {
		r := strings.NewReplacer(nameTemplate, cluster.Name, namespaceTemplate, cluster.Namespace)
		templated[k] = r.Replace(v)
	}
	return templated
}

// templateValues replaces Cluster.Name and Cluster.Namespace in Helm values text.
func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	clusterAutoscalerTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		Cluster *clusterv1.Cluster
	}

	templateInput := input{
		Cluster: cluster,
	}

	var b bytes.Buffer
	err = clusterAutoscalerTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf("failed setting target Cluster name and namespace in template: %w", err)
	}

	return b.String(), nil
}
