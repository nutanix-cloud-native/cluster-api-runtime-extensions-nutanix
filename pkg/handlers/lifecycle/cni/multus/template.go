// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/cni"
)

// templateValuesFunc returns a template function that parses the Multus values template,
// expands .ReadinessSocketPath placeholders, and ensures daemonConfig.readinessIndicatorFile
// is populated with the primary CNI readiness socket path when omitted or empty.
func templateValuesFunc(cniVar v1alpha1.CNI) func(*clusterv1.Cluster, string) (string, error) {
	return func(_ *clusterv1.Cluster, valuesTemplate string) (string, error) {
		readinessSocketPath, err := cni.ReadinessSocketPath(cniVar.Provider)
		if err != nil {
			return "", fmt.Errorf(
				"failed to get readiness socket path for CNI provider %s: %w",
				cniVar.Provider,
				err,
			)
		}

		t, err := template.New("").Parse(valuesTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to parse Multus values template: %w", err)
		}

		type input struct {
			ReadinessSocketPath string
		}

		var b bytes.Buffer
		if err = t.Execute(&b, input{ReadinessSocketPath: readinessSocketPath}); err != nil {
			return "", fmt.Errorf("failed to template Multus values: %w", err)
		}

		rendered := b.String()

		var parsed map[string]any
		if err = yaml.Unmarshal([]byte(rendered), &parsed); err != nil {
			return "", fmt.Errorf("failed to unmarshal rendered Multus values YAML: %w", err)
		}
		if parsed == nil {
			parsed = make(map[string]any)
		}

		var daemonConfig map[string]any
		if rawDaemonCfg, exists := parsed["daemonConfig"]; exists && rawDaemonCfg != nil {
			var ok bool
			daemonConfig, ok = rawDaemonCfg.(map[string]any)
			if !ok {
				return "", fmt.Errorf(
					"invalid Multus values: expected daemonConfig to be a map, got %T",
					rawDaemonCfg,
				)
			}
		} else {
			daemonConfig = make(map[string]any)
		}

		if rawSocket, exists := daemonConfig["readinessIndicatorFile"]; exists && rawSocket != nil {
			socketStr, ok := rawSocket.(string)
			if !ok {
				return "", fmt.Errorf(
					"invalid Multus values: expected daemonConfig.readinessIndicatorFile to be a string, got %T",
					rawSocket,
				)
			}
			if strings.TrimSpace(socketStr) != "" {
				return rendered, nil
			}
		}

		daemonConfig["readinessIndicatorFile"] = readinessSocketPath
		parsed["daemonConfig"] = daemonConfig

		marshaled, err := yaml.Marshal(parsed)
		if err != nil {
			return "", fmt.Errorf("failed to marshal Multus values YAML: %w", err)
		}

		return string(marshaled), nil
	}
}
