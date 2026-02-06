// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/builder"
)

func Test_templateValues(t *testing.T) {
	tests := []struct {
		name                           string
		cluster                        func(t *testing.T) *clusterv1.Cluster
		expectedRenderedValuesTemplate string
	}{
		{
			name: "EKS cluster with https prefix in controlPlaneEndpoint.Host",
			cluster: func(t *testing.T) *clusterv1.Cluster {
				return createTestCluster(
					t,
					"test-eks-cluster",
					"test-namespace",
					"eks",
					"https://test.eks.amazonaws.com",
					443,
				)
			},
			expectedRenderedValuesTemplate: expectedCiliumTemplateForEKS,
		},
		{
			name: "Non-EKS (Nutanix) cluster (should set ipam mode to kubernetes)",
			cluster: func(t *testing.T) *clusterv1.Cluster {
				return createTestCluster(t,
					"test-cluster",
					"test-namespace",
					"nutanix",
					"192.168.1.100",
					6443)
			},
			expectedRenderedValuesTemplate: expectedCiliumTemplateForNutanix,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := templateValues(tt.cluster(t), ciliumTemplate)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedRenderedValuesTemplate, got)
		})
	}
}

func Test_templateValues_TrimPrefixFunction(t *testing.T) {
	tests := []struct {
		name           string
		inputHost      string
		expectedOutput string
	}{
		{
			name:           "trim https prefix",
			inputHost:      "https://api.example.com",
			expectedOutput: "api.example.com",
		},
		{
			name:           "no prefix to trim",
			inputHost:      "api.example.com",
			expectedOutput: "api.example.com",
		},
		{
			name:           "trim https prefix with port",
			inputHost:      "https://api.example.com:8443",
			expectedOutput: "api.example.com:8443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := createTestCluster(t, "test-cluster", "test-namespace", "eks", tt.inputHost, 443)

			template := `k8sServiceHost: "{{ trimPrefix .ControlPlaneEndpoint.Host "https://" }}"`
			expected := `k8sServiceHost: "` + tt.expectedOutput + `"`

			got, err := templateValues(cluster, template)
			require.NoError(t, err)
			assert.Equal(t, expected, got)
		})
	}
}

// createTestCluster creates a test EKS cluster using ClusterBuilder
func createTestCluster(t *testing.T, name, namespace, provider, host string, port int32) *clusterv1.Cluster {
	// Create cluster config with kube-proxy disabled
	clusterConfigSpec := &apivariables.ClusterConfigSpec{
		KubeProxy: &carenv1.KubeProxy{
			Mode: carenv1.KubeProxyModeDisabled,
		},
	}

	// Marshal cluster config to cluster variable
	variable, err := apivariables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfigSpec)
	if err != nil {
		t.Fatalf("failed to marshal cluster config to cluster variable: %v", err)
	}

	topology := &clusterv1.Topology{
		Class:     "test-cluster-class",
		Version:   "v1.29.0",
		Variables: []clusterv1.ClusterVariable{*variable},
	}

	cluster := builder.Cluster(namespace, name).
		WithLabels(map[string]string{
			clusterv1.ProviderNameLabel: provider,
		}).
		WithTopology(topology).
		Build()

	// Set ControlPlaneEndpoint after building since ClusterBuilder doesn't support it
	cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: host,
		Port: port,
	}

	return cluster
}

const (
	// the template value is sourced from the Cilium values template in the project's helm chart
	ciliumTemplate = `
{{- if eq .Provider "eks" }}
ipam:
  mode: eni
{{- else }}
ipam:
  mode: kubernetes
{{- end }}

{{- if .EnableKubeProxyReplacement }}
kubeProxyReplacement: true
{{- end }}
k8sServiceHost: "{{ trimPrefix .ControlPlaneEndpoint.Host "https://" }}"
k8sServicePort: "{{ .ControlPlaneEndpoint.Port }}"
{{- if eq .Provider "eks" }}
enableIPv4Masquerade: false
eni:
  enabled: true
  awsReleaseExcessIPs: true
routingMode: native
endpointRoutes:
  enabled: true
{{- end }}
`
	expectedCiliumTemplateForEKS = `
ipam:
  mode: eni
kubeProxyReplacement: true
k8sServiceHost: "test.eks.amazonaws.com"
k8sServicePort: "443"
enableIPv4Masquerade: false
eni:
  enabled: true
  awsReleaseExcessIPs: true
routingMode: native
endpointRoutes:
  enabled: true
`

	expectedCiliumTemplateForNutanix = `
ipam:
  mode: kubernetes
kubeProxyReplacement: true
k8sServiceHost: "192.168.1.100"
k8sServicePort: "6443"
`
)
