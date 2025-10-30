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
			wantStatus: runtimehooksv1.ResponseStatus(""),
		},
		{
			name: "unsupported CNI provider skips deployment",
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
						Variables: []clusterv1.ClusterVariable{
							*testClusterVariable(t, &v1alpha1.CNI{
								Provider: "UnsupportedCNI",
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
