// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

func Test_shouldAutoDeployMultus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cloudProvider string
		cniProvider   string
		want          bool
	}{
		{
			name:          "nutanix cilium",
			cloudProvider: "nutanix",
			cniProvider:   v1alpha1.CNIProviderCilium,
			want:          true,
		},
		{
			name:          "nutanix calico",
			cloudProvider: "nutanix",
			cniProvider:   v1alpha1.CNIProviderCalico,
			want:          true,
		},
		{
			name:          "eks cilium",
			cloudProvider: "eks",
			cniProvider:   v1alpha1.CNIProviderCilium,
			want:          true,
		},
		{
			name:          "nutanix flow ships its own multus",
			cloudProvider: "nutanix",
			cniProvider:   v1alpha1.CNIProviderFlow,
			want:          false,
		},
		{
			name:          "docker cilium is unsupported",
			cloudProvider: "docker",
			cniProvider:   v1alpha1.CNIProviderCilium,
			want:          false,
		},
		{
			name:          "empty cni",
			cloudProvider: "nutanix",
			cniProvider:   "",
			want:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldAutoDeployMultus(tt.cloudProvider, tt.cniProvider)
			if got != tt.want {
				t.Fatalf("shouldAutoDeployMultus(%q, %q) = %v, want %v",
					tt.cloudProvider, tt.cniProvider, got, tt.want)
			}
		})
	}
}

func TestAfterControlPlaneInitialized_skipsFlowCNI(t *testing.T) {
	t.Parallel()

	cv, err := apivariables.MarshalToClusterVariable(
		v1alpha1.ClusterConfigVariableName,
		&apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				CNI: &v1alpha1.CNI{
					Provider: v1alpha1.CNIProviderFlow,
					Strategy: v1alpha1.AddonStrategyHelmAddon,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("failed to marshal cluster variable: %v", err)
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1ObjectMetaWithProvider("nutanix"),
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "dummy-class"},
				Variables: []clusterv1.ClusterVariable{
					*cv,
				},
			},
		},
	}
	clusterV1beta1, err := capiutils.ConvertV1Beta2ClusterToV1Beta1(cluster)
	if err != nil {
		t.Fatalf("failed to convert cluster: %v", err)
	}

	handler := New(
		fake.NewClientBuilder().Build(),
		NewMultusConfig(&options.GlobalOptions{}),
		nil,
	)
	resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
	handler.AfterControlPlaneInitialized(
		context.Background(),
		&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *clusterV1beta1},
		resp,
	)

	if diff := cmp.Diff(runtimehooksv1.ResponseStatus(""), resp.Status); diff != "" {
		t.Errorf("response Status mismatch (-want +got):\n%s. Message: %s", diff, resp.Message)
	}
}

func metav1ObjectMetaWithProvider(provider string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      "test-cluster",
		Namespace: "default",
		Labels: map[string]string{
			clusterv1.ProviderNameLabel: provider,
		},
	}
}
