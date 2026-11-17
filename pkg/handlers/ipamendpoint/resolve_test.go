// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ipamendpoint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	clusterName      = "workload"
	clusterNamespace = "default"
	reserved         = "10.0.0.7"
)

func clusterKey() ctrlclient.ObjectKey {
	return ctrlclient.ObjectKey{Namespace: clusterNamespace, Name: clusterName}
}

func clientWithAddress(address string) ctrlclient.Client {
	scheme := runtime.NewScheme()
	utilruntime.Must(ipamv1.AddToScheme(scheme))
	builder := fake.NewClientBuilder().WithScheme(scheme)
	if address != "" {
		builder = builder.WithObjects(&ipamv1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClaimName(clusterName),
				Namespace: clusterNamespace,
			},
			Spec: ipamv1.IPAddressSpec{Address: address},
		})
	}
	return builder.Build()
}

// reservingSpec is the shape a cluster has when it asks for its API server
// address to be reserved: no host, and an ipReservation naming the subnet.
func reservingSpec() *v1alpha1.ControlPlaneEndpointSpec {
	return &v1alpha1.ControlPlaneEndpointSpec{
		Port: 6443,
		VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
			Configuration: &v1alpha1.ControlPlaneVirtualIPConfiguration{
				IPReservation: &v1alpha1.ControlPlaneIPReservation{Subnet: "vlan170"},
			},
		},
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("fills in host and the virtual IP from the reserved address", func(t *testing.T) {
		t.Parallel()
		spec := reservingSpec()
		ok, err := Resolve(context.Background(), clientWithAddress(reserved), clusterKey(), spec)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, reserved, spec.Host)
		assert.Equal(t, reserved, spec.VirtualIPSpec.Configuration.Address)
	})

	// The gap this closes: the address is never written back into the Cluster,
	// because NKP forbids it, so a handler that only read the variable would
	// patch the endpoint in as empty.
	t.Run("an unresolved spec would leave the endpoint empty", func(t *testing.T) {
		t.Parallel()
		spec := reservingSpec()
		assert.Empty(t, spec.Host, "precondition: the variable carries no host")
		ok, err := Resolve(context.Background(), clientWithAddress(reserved), clusterKey(), spec)
		require.NoError(t, err)
		require.True(t, ok)
		assert.NotEmpty(t, spec.Host)
	})

	t.Run("a host the operator chose is never overwritten", func(t *testing.T) {
		t.Parallel()
		spec := reservingSpec()
		spec.Host = "api.example.com"
		ok, err := Resolve(context.Background(), clientWithAddress(reserved), clusterKey(), spec)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "api.example.com", spec.Host, "an external endpoint must survive")
		assert.Equal(t, reserved, spec.VirtualIPSpec.Configuration.Address,
			"the reserved address becomes the virtual IP instead")
	})

	t.Run("an explicit virtual IP is never overwritten", func(t *testing.T) {
		t.Parallel()
		spec := reservingSpec()
		spec.VirtualIPSpec.Configuration.Address = "10.0.0.99"
		_, err := Resolve(context.Background(), clientWithAddress(reserved), clusterKey(), spec)
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.99", spec.VirtualIPSpec.Configuration.Address)
	})

	t.Run("a cluster that does not reserve is left alone", func(t *testing.T) {
		t.Parallel()
		spec := &v1alpha1.ControlPlaneEndpointSpec{Host: "10.0.0.1", Port: 6443}
		ok, err := Resolve(context.Background(), clientWithAddress(reserved), clusterKey(), spec)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, "10.0.0.1", spec.Host)
	})

	t.Run("no IPAddress yet is not an error", func(t *testing.T) {
		t.Parallel()
		spec := reservingSpec()
		ok, err := Resolve(context.Background(), clientWithAddress(""), clusterKey(), spec)
		require.NoError(t, err, "BeforeClusterCreate is what gates on the address, not this")
		assert.False(t, ok)
		assert.Empty(t, spec.Host)
	})

	t.Run("an IPAddress that exists but carries no address is not resolved", func(t *testing.T) {
		t.Parallel()
		scheme := runtime.NewScheme()
		utilruntime.Must(ipamv1.AddToScheme(scheme))
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&ipamv1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClaimName(clusterName),
				Namespace: clusterNamespace,
			},
		}).Build()
		spec := reservingSpec()
		ok, err := Resolve(context.Background(), c, clusterKey(), spec)
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Empty(t, spec.Host, "an empty address must not be injected as the endpoint")
	})

	t.Run("a nil spec is tolerated", func(t *testing.T) {
		t.Parallel()
		ok, err := Resolve(context.Background(), clientWithAddress(reserved), clusterKey(), nil)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("the claim name is derived from the cluster name", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "workload-control-plane-endpoint", ClaimName("workload"))
		assert.Equal(t, ClaimName("workload"), ClaimName("workload"))
	})
}
