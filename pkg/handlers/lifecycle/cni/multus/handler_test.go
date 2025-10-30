// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

func testClusterVariable(t *testing.T, cni *v1alpha1.CNI) *clusterv1.ClusterVariable {
	t.Helper()
	cv, err := apivariables.MarshalToClusterVariable(
		"clusterConfig",
		&apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				GenericAddons: v1alpha1.GenericAddons{
					CNI: cni,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("failed to create clusterVariable: %s", err)
	}
	return cv
}

func TestAfterControlPlaneInitialized(t *testing.T) {
	tests := []struct {
		name           string
		cluster        clusterv1.Cluster
		setupClient    func() ctrlclient.Client
		wantStatus     runtimehooksv1.ResponseStatus
		wantFailureMsg bool
	}{
		{
			name: "unsupported cloud provider skips deployment",
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "DockerCluster",
					},
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							*testClusterVariable(t, &v1alpha1.CNI{
								Provider: v1alpha1.CNIProviderCilium,
							}),
						},
					},
				},
			},
			setupClient: func() ctrlclient.Client {
				return fake.NewClientBuilder().Build()
			},
			wantStatus: runtimehooksv1.ResponseStatus(""),
		},
		{
			name: "no CNI configured skips deployment",
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSManagedCluster",
					},
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{},
					},
				},
			},
			setupClient: func() ctrlclient.Client {
				return fake.NewClientBuilder().Build()
			},
			wantStatus: runtimehooksv1.ResponseStatus(""),
		},
		{
			name: "unsupported CNI provider returns failure",
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "eks",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSManagedCluster",
					},
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							*testClusterVariable(t, &v1alpha1.CNI{
								Provider: "UnsupportedCNI",
							}),
						},
					},
				},
			},
			setupClient: func() ctrlclient.Client {
				// Create ConfigMaps needed for Multus deployment
				valuesTemplateCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default-multus-values-template",
						Namespace: "default",
					},
					Data: map[string]string{
						"values.yaml": `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
`,
					},
				}
				helmConfigCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "helm-config",
						Namespace: "default",
					},
					Data: map[string]string{
						"multus": `ChartName: multus
ChartVersion: 0.1.0
RepositoryURL: 'oci://helm-repository.default.svc/charts'`,
					},
				}
				return fake.NewClientBuilder().
					WithObjects(valuesTemplateCM, helmConfigCM).
					Build()
			},
			// Unsupported CNI provider causes templateValuesFunc to fail,
			// which causes Apply to fail, returning Failure status
			wantStatus:     runtimehooksv1.ResponseStatusFailure,
			wantFailureMsg: true,
		},
		{
			name: "EKS cluster with Cilium CNI - deployment fails without cluster UUID",
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "eks",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSManagedCluster",
					},
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							*testClusterVariable(t, &v1alpha1.CNI{
								Provider: v1alpha1.CNIProviderCilium,
							}),
						},
					},
				},
			},
			setupClient: func() ctrlclient.Client {
				// Create ConfigMaps needed for Multus deployment
				valuesTemplateCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default-multus-values-template",
						Namespace: "default",
					},
					Data: map[string]string{
						"values.yaml": `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
`,
					},
				}
				helmConfigCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "helm-config",
						Namespace: "default",
					},
					Data: map[string]string{
						"multus": `ChartName: multus
ChartVersion: 0.1.0
RepositoryURL: 'oci://helm-repository.default.svc/charts'`,
					},
				}
				return fake.NewClientBuilder().
					WithObjects(valuesTemplateCM, helmConfigCM).
					Build()
			},
			// In unit tests, deployment fails due to missing cluster UUID annotation
			// which is expected - the handler should return failure status
			wantStatus:     runtimehooksv1.ResponseStatusFailure,
			wantFailureMsg: true,
		},
		{
			name: "EKS cluster with Calico CNI - deployment fails without cluster UUID",
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "eks",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSManagedCluster",
					},
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							*testClusterVariable(t, &v1alpha1.CNI{
								Provider: v1alpha1.CNIProviderCalico,
							}),
						},
					},
				},
			},
			setupClient: func() ctrlclient.Client {
				valuesTemplateCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default-multus-values-template",
						Namespace: "default",
					},
					Data: map[string]string{
						"values.yaml": `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
`,
					},
				}
				helmConfigCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "helm-config",
						Namespace: "default",
					},
					Data: map[string]string{
						"multus": `ChartName: multus
ChartVersion: 0.1.0
RepositoryURL: 'oci://helm-repository.default.svc/charts'`,
					},
				}
				return fake.NewClientBuilder().
					WithObjects(valuesTemplateCM, helmConfigCM).
					Build()
			},
			wantStatus:     runtimehooksv1.ResponseStatusFailure,
			wantFailureMsg: true,
		},
		{
			name: "missing HelmChart config returns failure",
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "eks",
					},
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "AWSManagedCluster",
					},
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							*testClusterVariable(t, &v1alpha1.CNI{
								Provider: v1alpha1.CNIProviderCilium,
							}),
						},
					},
				},
			},
			setupClient: func() ctrlclient.Client {
				// No ConfigMaps, so HelmChartGetter will fail
				return fake.NewClientBuilder().Build()
			},
			wantStatus:     runtimehooksv1.ResponseStatusFailure,
			wantFailureMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := tt.setupClient()
			globalOptions := options.NewGlobalOptions()
			multusConfig := NewMultusConfig(globalOptions)
			helmChartGetter := config.NewHelmChartGetterFromConfigMap("helm-config", "default", client)
			handler := New(client, multusConfig, helmChartGetter)

			resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

			req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
				Cluster: tt.cluster,
			}

			handler.AfterControlPlaneInitialized(ctx, req, resp)
			if diff := cmp.Diff(tt.wantStatus, resp.Status); diff != "" {
				t.Errorf(
					"response Status mismatch (-want +got):\n%s. Message: %s",
					diff,
					resp.Message,
				)
			}
			if tt.wantFailureMsg && resp.Message == "" {
				t.Error("expected failure message but got empty message")
			}
		})
	}
}

func TestBeforeClusterUpgrade(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	globalOptions := options.NewGlobalOptions()
	multusConfig := NewMultusConfig(globalOptions)
	helmChartGetter := config.NewHelmChartGetterFromConfigMap("helm-config", "default", client)
	handler := New(client, multusConfig, helmChartGetter)

	tests := []struct {
		name       string
		cluster    clusterv1.Cluster
		wantStatus runtimehooksv1.ResponseStatus
	}{
		{
			name: "unsupported cloud provider skips deployment",
			cluster: clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					InfrastructureRef: &corev1.ObjectReference{
						Kind: "DockerCluster",
					},
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							*testClusterVariable(t, &v1alpha1.CNI{
								Provider: v1alpha1.CNIProviderCilium,
							}),
						},
					},
				},
			},
			wantStatus: runtimehooksv1.ResponseStatus(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			resp := &runtimehooksv1.BeforeClusterUpgradeResponse{}

			req := &runtimehooksv1.BeforeClusterUpgradeRequest{
				Cluster: tt.cluster,
			}

			handler.BeforeClusterUpgrade(ctx, req, resp)
			if diff := cmp.Diff(tt.wantStatus, resp.Status); diff != "" {
				t.Errorf(
					"response Status mismatch (-want +got):\n%s. Message: %s",
					diff,
					resp.Message,
				)
			}
		})
	}
}
