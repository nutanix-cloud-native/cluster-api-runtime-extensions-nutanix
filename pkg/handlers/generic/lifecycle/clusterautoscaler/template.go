// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import "strings"

const (
	nameTemplate      = "tmpl-clustername-tmpl"
	namespaceTemplate = "tmpl-clusternamespace-tmpl"
)

// templateData replaces templates 'tmpl-clustername-tmpl' and 'tmpl-clusternamespace-tmpl' in data
// with clusterName and clusterNamespace.
func templateData(data map[string]string, clusterName, clusterNamespace string) map[string]string {
	templated := make(map[string]string, len(data))
	for k, v := range data {
		r := strings.NewReplacer(nameTemplate, clusterName, namespaceTemplate, clusterNamespace)
		templated[k] = r.Replace(v)
	}
	return templated
}
