// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
)

func Test_templateValues(t *testing.T) {
	tests := []struct {
		name                           string
		cluster                        *clusterv1.Cluster
		expectedRenderedValuesTemplate string
	}{
		{
			name: "EKS cluster with https prefix in controlPlaneEndpoint.Host",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-eks-cluster",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"cluster.x-k8s.io/provider": "eks",
					},
				},
				Spec: clusterv1.ClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "https://test.eks.amazonaws.com",
						Port: 443,
					},
					Topology: &clusterv1.Topology{
						ControlPlane: clusterv1.ControlPlaneTopology{
							Metadata: clusterv1.ObjectMeta{
								Annotations: map[string]string{
									controlplanev1.SkipKubeProxyAnnotation: "",
								},
							},
						},
					},
				},
			},
			expectedRenderedValuesTemplate: expectedCiliumTemplateForEKS,
		},
		{
			name: "Non-EKS (Nutanix) cluster (should use auto for controlPlaneEndpointHost)",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"cluster.x-k8s.io/provider": "nutanix",
					},
				},
				Spec: clusterv1.ClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "192.168.1.100",
						Port: 6443,
					},
					Topology: &clusterv1.Topology{
						ControlPlane: clusterv1.ControlPlaneTopology{
							Metadata: clusterv1.ObjectMeta{
								Annotations: map[string]string{
									controlplanev1.SkipKubeProxyAnnotation: "",
								},
							},
						},
					},
				},
			},
			expectedRenderedValuesTemplate: expectedCiliumTemplateForNutanix,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := templateValues(tt.cluster, ciliumTemplate)
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
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"cluster.x-k8s.io/provider": "eks",
					},
				},
				Spec: clusterv1.ClusterSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: tt.inputHost,
						Port: 443,
					},
				},
			}

			template := `k8sServiceHost: "{{ trimPrefix .Cluster.Spec.ControlPlaneEndpoint.Host "https://" }}"`
			expected := `k8sServiceHost: "` + tt.expectedOutput + `"`

			got, err := templateValues(cluster, template)
			require.NoError(t, err)
			assert.Equal(t, expected, got)
		})
	}
}

const (
	// the template value is sourced from the Cilium values template in the project's helm chart
	ciliumTemplate = `
{{- $capiProvider := index .Cluster.Labels "cluster.x-k8s.io/provider" }}
{{- if eq $capiProvider "eks" }}
ipam:
  mode: eni
{{- else }}
ipam:
  mode: kubernetes
{{- end }}

{{- if .EnableKubeProxyReplacement }}
kubeProxyReplacement: true
{{- end }}

{{- if eq $capiProvider "eks" }}
k8sServiceHost: "{{ trimPrefix .Cluster.Spec.ControlPlaneEndpoint.Host "https://" }}"
k8sServicePort: "{{ .Cluster.Spec.ControlPlaneEndpoint.Port }}"
{{- else }}
k8sServiceHost: auto
{{- end }}

{{- if eq $capiProvider "eks" }}
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
k8sServiceHost: auto
`
)
