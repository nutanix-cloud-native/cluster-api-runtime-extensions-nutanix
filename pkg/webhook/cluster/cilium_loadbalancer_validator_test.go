// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func newCiliumSLBCluster(
	t *testing.T,
	cni string,
	kp v1alpha1.KubeProxyMode,
	slbProvider string,
) *clusterv1beta2.Cluster {
	t.Helper()
	spec := &variables.ClusterConfigSpec{
		KubeProxy: &v1alpha1.KubeProxy{Mode: kp},
		Addons: &variables.Addons{
			GenericAddons: v1alpha1.GenericAddons{
				ServiceLoadBalancer: &v1alpha1.ServiceLoadBalancer{Provider: slbProvider},
			},
			CNI: &v1alpha1.CNI{Provider: cni},
		},
	}
	raw, err := json.Marshal(spec)
	require.NoError(t, err)
	return &clusterv1beta2.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Spec: clusterv1beta2.ClusterSpec{
			Topology: clusterv1beta2.Topology{
				ClassRef: clusterv1beta2.ClusterClassRef{Name: "cc"},
				Version:  "v1.30.0",
				Variables: []clusterv1beta2.ClusterVariable{{
					Name:  v1alpha1.ClusterConfigVariableName,
					Value: apiextensionsv1.JSON{Raw: raw},
				}},
			},
		},
	}
}

func TestCiliumLoadBalancerValidator(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1beta2.AddToScheme(scheme))

	tests := []struct {
		name        string
		cluster     *clusterv1beta2.Cluster
		allowed     bool
		errContains string
	}{
		{
			name: "provider MetalLB with Cilium CNI is allowed",
			cluster: newCiliumSLBCluster(
				t,
				v1alpha1.CNIProviderCilium,
				v1alpha1.KubeProxyModeDisabled,
				v1alpha1.ServiceLoadBalancerProviderMetalLB,
			),
			allowed: true,
		},
		{
			name: "provider Cilium with Cilium CNI and kube-proxy disabled is allowed",
			cluster: newCiliumSLBCluster(
				t,
				v1alpha1.CNIProviderCilium,
				v1alpha1.KubeProxyModeDisabled,
				v1alpha1.ServiceLoadBalancerProviderCilium,
			),
			allowed: true,
		},
		{
			name: "provider Cilium with Calico CNI is denied",
			cluster: newCiliumSLBCluster(
				t,
				v1alpha1.CNIProviderCalico,
				v1alpha1.KubeProxyModeDisabled,
				v1alpha1.ServiceLoadBalancerProviderCilium,
			),
			allowed:     false,
			errContains: "Cilium CNI",
		},
		{
			name: "provider Cilium with kube-proxy iptables is denied",
			cluster: newCiliumSLBCluster(
				t,
				v1alpha1.CNIProviderCilium,
				v1alpha1.KubeProxyModeIPTables,
				v1alpha1.ServiceLoadBalancerProviderCilium,
			),
			allowed:     false,
			errContains: "kube-proxy",
		},
		{
			name: "skip-all annotation bypasses validation",
			cluster: func() *clusterv1beta2.Cluster {
				c := newCiliumSLBCluster(
					t,
					v1alpha1.CNIProviderCalico,
					v1alpha1.KubeProxyModeIPTables,
					v1alpha1.ServiceLoadBalancerProviderCilium,
				)
				c.Annotations = map[string]string{
					v1alpha1.PreflightChecksSkipAnnotationKey: v1alpha1.PreflightChecksSkipAllAnnotationValue,
				}
				return c
			}(),
			allowed: true,
		},
	}

	decoder := admission.NewDecoder(scheme)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).Build()
			v := NewCiliumLoadBalancerValidator(c, decoder)
			raw, err := json.Marshal(tc.cluster)
			require.NoError(t, err)
			req := admission.Request{AdmissionRequest: v1.AdmissionRequest{
				Operation: v1.Create,
				Object:    runtime.RawExtension{Raw: raw},
			}}
			resp := v.validate(context.Background(), req)
			assert.Equal(t, tc.allowed, resp.Allowed)
			if !tc.allowed {
				require.NotNil(t, resp.Result)
				assert.Contains(t, resp.Result.Message, tc.errContains)
			}
		})
	}
}

func TestCiliumLoadBalancerValidator_ProviderChangeOnUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1beta2.AddToScheme(scheme))

	newCluster := newCiliumSLBCluster(
		t,
		v1alpha1.CNIProviderCilium,
		v1alpha1.KubeProxyModeDisabled,
		v1alpha1.ServiceLoadBalancerProviderCilium,
	)
	oldCluster := newCiliumSLBCluster(
		t,
		v1alpha1.CNIProviderCilium,
		v1alpha1.KubeProxyModeDisabled,
		v1alpha1.ServiceLoadBalancerProviderMetalLB,
	)

	newRaw, err := json.Marshal(newCluster)
	require.NoError(t, err)
	oldRaw, err := json.Marshal(oldCluster)
	require.NoError(t, err)

	decoder := admission.NewDecoder(scheme)
	v := NewCiliumLoadBalancerValidator(
		fake.NewClientBuilder().WithScheme(scheme).Build(),
		decoder,
	)
	req := admission.Request{AdmissionRequest: v1.AdmissionRequest{
		Operation: v1.Update,
		Object:    runtime.RawExtension{Raw: newRaw},
		OldObject: runtime.RawExtension{Raw: oldRaw},
	}}
	resp := v.validate(context.Background(), req)
	assert.False(t, resp.Allowed)
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "provider change")
}

func TestCiliumLoadBalancerValidator_DeleteIsAllowed(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1beta2.AddToScheme(scheme))
	decoder := admission.NewDecoder(scheme)
	v := NewCiliumLoadBalancerValidator(fake.NewClientBuilder().Build(), decoder)
	resp := v.validate(
		context.Background(),
		admission.Request{AdmissionRequest: v1.AdmissionRequest{Operation: v1.Delete}},
	)
	assert.True(t, resp.Allowed)
}
