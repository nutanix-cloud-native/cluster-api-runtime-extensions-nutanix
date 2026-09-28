// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ciliumv2 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumv2alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestConfigurationObjects_EmptyRangesReturnsError(t *testing.T) {
	t.Parallel()

	_, err := ConfigurationObjects(&ConfigurationInput{Name: "caren", Namespace: "kube-system"})
	require.Error(t, err)
}

func TestConfigurationObjects_ProducesPoolAndPolicy(t *testing.T) {
	t.Parallel()

	in := &ConfigurationInput{
		Name:      "caren",
		Namespace: "kube-system",
		AddressRanges: []v1alpha1.AddressRange{
			{Start: "10.0.0.10", End: "10.0.0.20"},
			{Start: "10.0.0.50", End: "10.0.0.60"},
		},
	}

	objs, err := ConfigurationObjects(in)
	require.NoError(t, err)
	require.Len(t, objs, 2)

	pool, ok := objs[0].(*ciliumv2.CiliumLoadBalancerIPPool)
	require.True(t, ok, "first object must be CiliumLoadBalancerIPPool, got %T", objs[0])
	assert.Equal(t, "caren", pool.Name)
	assert.Equal(t, "kube-system", pool.Namespace)
	assert.Equal(t, "CiliumLoadBalancerIPPool", pool.Kind)
	assert.Equal(t, ciliumv2.SchemeGroupVersion.String(), pool.APIVersion)
	require.Len(t, pool.Spec.Blocks, 2)
	assert.Equal(t, "10.0.0.10", pool.Spec.Blocks[0].Start)
	assert.Equal(t, "10.0.0.20", pool.Spec.Blocks[0].Stop)
	assert.Equal(t, "10.0.0.50", pool.Spec.Blocks[1].Start)
	assert.Equal(t, "10.0.0.60", pool.Spec.Blocks[1].Stop)

	policy, ok := objs[1].(*ciliumv2alpha1.CiliumL2AnnouncementPolicy)
	require.True(t, ok, "second object must be CiliumL2AnnouncementPolicy, got %T", objs[1])
	assert.Equal(t, "caren", policy.Name)
	assert.Equal(t, "kube-system", policy.Namespace)
	assert.Equal(t, "CiliumL2AnnouncementPolicy", policy.Kind)
	assert.Equal(t, ciliumv2alpha1.SchemeGroupVersion.String(), policy.APIVersion)
	assert.True(t, policy.Spec.LoadBalancerIPs, "LoadBalancerIPs must be true")
	assert.False(t, policy.Spec.ExternalIPs, "ExternalIPs must be false by default")
}
