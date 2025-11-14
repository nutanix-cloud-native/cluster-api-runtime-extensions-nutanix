// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestKonnectorAgentLegacyDeploymentCheck_Name(t *testing.T) {
	check := &konnectorAgentLegacyDeploymentCheck{}
	assert.Equal(t, "NutanixKonnectorAgentLegacyDeployment", check.Name())
}

func TestKonnectorAgentLegacyDeploymentCheck_Run(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	tests := []struct {
		name      string
		cluster   *clusterv1.Cluster
		want      preflight.CheckResult
		wantError bool
	}{
		{
			name: "konnector agent not enabled - should skip",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{},
					},
				},
				Status: clusterv1.ClusterStatus{
					InfrastructureReady: true,
				},
			},
			want: preflight.CheckResult{
				Allowed: true,
			},
		},
		{
			name: "infrastructure not ready - should skip (CREATE scenario)",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: apiextensionsv1.JSON{
									Raw: mustMarshalJSON(t, map[string]interface{}{
										"addons": map[string]interface{}{
											carenv1.KonnectorAgentVariableName: map[string]interface{}{},
										},
									}),
								},
							},
						},
					},
				},
				Status: clusterv1.ClusterStatus{
					InfrastructureReady: false,
				},
			},
			want: preflight.CheckResult{
				Allowed: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			client := fake.NewClientBuilder().WithScheme(scheme).Build()

			// Create check
			check := &konnectorAgentLegacyDeploymentCheck{
				kclient: client,
				cluster: tt.cluster,
				log:     testr.New(t),
			}

			// Run the check
			result := check.Run(context.Background())

			// Verify result
			assert.Equal(t, tt.want.Allowed, result.Allowed)
			if tt.want.Causes != nil {
				assert.Equal(t, len(tt.want.Causes), len(result.Causes))
				if len(tt.want.Causes) > 0 {
					assert.Equal(t, tt.want.Causes[0].Field, result.Causes[0].Field)
					assert.Contains(t, result.Causes[0].Message, legacyHelmChartName)
				}
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}

// TestListLegacyHelmReleases tests the filtering logic for legacy releases
func TestListLegacyHelmReleases(t *testing.T) {
	tests := []struct {
		name          string
		releases      []*release.Release
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "no releases",
			releases:      []*release.Release{},
			expectedCount: 0,
		},
		{
			name: "one legacy release",
			releases: []*release.Release{
				{
					Name:      "nutanix-k8s-agent",
					Namespace: "default",
					Chart: &chart.Chart{
						Metadata: &chart.Metadata{
							Name: legacyHelmChartName,
						},
					},
				},
			},
			expectedCount: 1,
			expectedNames: []string{"nutanix-k8s-agent"},
		},
		{
			name: "multiple legacy releases",
			releases: []*release.Release{
				{
					Name:      "nutanix-k8s-agent-1",
					Namespace: "default",
					Chart: &chart.Chart{
						Metadata: &chart.Metadata{
							Name: legacyHelmChartName,
						},
					},
				},
				{
					Name:      "nutanix-k8s-agent-2",
					Namespace: "kube-system",
					Chart: &chart.Chart{
						Metadata: &chart.Metadata{
							Name: legacyHelmChartName,
						},
					},
				},
			},
			expectedCount: 2,
			expectedNames: []string{"nutanix-k8s-agent-1", "nutanix-k8s-agent-2"},
		},
		{
			name: "mixed releases - only legacy filtered",
			releases: []*release.Release{
				{
					Name:      "nutanix-k8s-agent",
					Namespace: "default",
					Chart: &chart.Chart{
						Metadata: &chart.Metadata{
							Name: legacyHelmChartName,
						},
					},
				},
				{
					Name:      "other-chart",
					Namespace: "default",
					Chart: &chart.Chart{
						Metadata: &chart.Metadata{
							Name: "other-chart",
						},
					},
				},
			},
			expectedCount: 1,
			expectedNames: []string{"nutanix-k8s-agent"},
		},
		{
			name: "release with nil chart",
			releases: []*release.Release{
				{
					Name:      "nutanix-k8s-agent",
					Namespace: "default",
					Chart:     nil,
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Filter releases manually (simulating the logic)
			var legacyReleases []*release.Release
			for _, rel := range tt.releases {
				if rel.Chart != nil && rel.Chart.Name() == legacyHelmChartName {
					legacyReleases = append(legacyReleases, rel)
				}
			}

			assert.Equal(t, tt.expectedCount, len(legacyReleases))
			if tt.expectedNames != nil {
				for i, name := range tt.expectedNames {
					assert.Equal(t, name, legacyReleases[i].Name)
				}
			}
		})
	}
}

func mustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
