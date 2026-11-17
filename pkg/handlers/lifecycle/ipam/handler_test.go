// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	clusterName      = "workload"
	clusterNamespace = "default"
	prismCentralHost = "prism-central.nutanix.com"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(ipamv1.AddToScheme(scheme))
	// The pool is created unstructured, so the fake client needs to know the
	// list kind to track it.
	scheme.AddKnownTypeWithName(nutanixIPPoolGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		nutanixIPPoolGVK.GroupVersion().WithKind(nutanixIPPoolGVK.Kind+"List"),
		&unstructured.UnstructuredList{},
	)
	return scheme
}

// testCluster builds a cluster carrying the clusterConfig topology variable.
// host is the configured API server address, which is empty when it is to be
// reserved. reservation is nil for a cluster that names its own address.
func testCluster(t *testing.T, host string, reservation *carenv1.ControlPlaneIPReservation) *clusterv1.Cluster {
	t.Helper()

	nutanix := &carenv1.NutanixSpec{
		PrismCentralEndpoint: carenv1.NutanixPrismCentralEndpointSpec{
			URL: fmt.Sprintf("https://%s:9440", prismCentralHost),
			Credentials: carenv1.NutanixPrismCentralEndpointCredentials{
				SecretRef: carenv1.LocalObjectReference{Name: "creds"},
			},
			Insecure: true,
		},
		ControlPlaneEndpoint: carenv1.ControlPlaneEndpointSpec{
			Host: host,
			Port: 6443,
		},
	}
	if reservation != nil {
		nutanix.ControlPlaneEndpoint.VirtualIPSpec = &carenv1.ControlPlaneVirtualIPSpec{
			Provider: "KubeVIP",
			Configuration: &carenv1.ControlPlaneVirtualIPConfiguration{
				IPReservation: reservation,
			},
		}
	}

	variable, err := apivariables.MarshalToClusterVariable(
		carenv1.ClusterConfigVariableName,
		&apivariables.ClusterConfigSpec{Nutanix: nutanix},
	)
	require.NoError(t, err)

	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterNamespace,
			UID:       types.UID("0f1b17bc-289f-4cc2-b558-6eef10065e57"),
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				Variables: []clusterv1.ClusterVariable{*variable},
			},
		},
	}
}

func allocatedClaim(address string) []ctrlclient.Object {
	return []ctrlclient.Object{
		&ipamv1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      poolName(clusterName),
				Namespace: clusterNamespace,
			},
			Status: ipamv1.IPAddressClaimStatus{
				AddressRef: ipamv1.IPAddressReference{Name: "allocated"},
			},
		},
		&ipamv1.IPAddress{
			ObjectMeta: metav1.ObjectMeta{Name: "allocated", Namespace: clusterNamespace},
			Spec:       ipamv1.IPAddressSpec{Address: address},
		},
	}
}

// configuredHost returns the API server address recorded in the cluster's
// topology variable, which is what kube-vip and the CNI are templated from.
func configuredHost(t *testing.T, c ctrlclient.Client) string {
	t.Helper()
	live := &clusterv1.Cluster{}
	require.NoError(t, c.Get(context.Background(),
		ctrlclient.ObjectKey{Name: clusterName, Namespace: clusterNamespace}, live))
	clusterConfig, err := apivariables.UnmarshalClusterConfigVariable(live.Spec.Topology.Variables)
	require.NoError(t, err)
	require.NotNil(t, clusterConfig.Nutanix)
	return clusterConfig.Nutanix.ControlPlaneEndpoint.Host
}

// configuredVirtualIP returns the address kube-vip is templated with.
func configuredVirtualIP(t *testing.T, c ctrlclient.Client) string {
	t.Helper()
	live := &clusterv1.Cluster{}
	require.NoError(t, c.Get(context.Background(),
		ctrlclient.ObjectKey{Name: clusterName, Namespace: clusterNamespace}, live))
	clusterConfig, err := apivariables.UnmarshalClusterConfigVariable(live.Spec.Topology.Variables)
	require.NoError(t, err)
	virtualIP := clusterConfig.Nutanix.ControlPlaneEndpoint.VirtualIPSpec
	if virtualIP == nil || virtualIP.Configuration == nil {
		return ""
	}
	return virtualIP.Configuration.Address
}

// A cluster that names its own address must not create any IPAM object: this
// is every cluster that exists today.
func TestBeforeClusterCreateStaticHostIsUntouched(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "10.0.0.5", nil)
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(cluster).Build()

	resp := &runtimehooksv1.BeforeClusterCreateResponse{}
	New(c).BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)
	assert.Equal(t, "10.0.0.5", configuredHost(t, c))

	claim := &ipamv1.IPAddressClaim{}
	err := c.Get(context.Background(),
		ctrlclient.ObjectKey{Name: poolName(clusterName), Namespace: clusterNamespace}, claim)
	assert.True(t, err != nil, "a claim was created for a cluster that names its own address")
}

// The first pass creates the pool and the claim and reports a retry, because
// the provider cannot have allocated an address yet.
func TestBeforeClusterCreateFirstPassCreatesAndRetries(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "", &carenv1.ControlPlaneIPReservation{
		Subnet:  "vlan170",
		Cluster: ptr.To("pe-1"),
	})
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(cluster).Build()

	resp := &runtimehooksv1.BeforeClusterCreateResponse{}
	New(c).BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
	assert.Equal(t, int32(retryAfterSeconds), resp.RetryAfterSeconds,
		"the retry must be set on the typed response")
	assert.Contains(t, resp.Message, "waiting for the API server address")

	ctx := context.Background()
	key := ctrlclient.ObjectKey{Name: poolName(clusterName), Namespace: clusterNamespace}

	pool := &unstructured.Unstructured{}
	pool.SetGroupVersionKind(nutanixIPPoolGVK)
	require.NoError(t, c.Get(ctx, key, pool))
	spec, found, err := unstructured.NestedMap(pool.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "vlan170", spec["subnet"])
	assert.Equal(t, "pe-1", spec["cluster"])
	prismCentral, _, err := unstructured.NestedMap(pool.Object, "spec", "prismCentral")
	require.NoError(t, err)
	assert.Equal(t, prismCentralHost, prismCentral["address"])
	assert.Equal(t, int64(9440), prismCentral["port"])
	assert.Equal(t, true, prismCentral["insecure"])

	claim := &ipamv1.IPAddressClaim{}
	require.NoError(t, c.Get(ctx, key, claim))
	assert.Equal(t, clusterName, claim.Spec.ClusterName)
	assert.Equal(t, poolName(clusterName), claim.Spec.PoolRef.Name)
	assert.Equal(t, "NutanixIPPool", claim.Spec.PoolRef.Kind)
	assert.Equal(t, "ipam.cluster.x-k8s.io", claim.Spec.PoolRef.APIGroup)
	// The owner reference is what releases the address when the cluster goes.
	require.Len(t, claim.OwnerReferences, 1)
	assert.Equal(t, "Cluster", claim.OwnerReferences[0].Kind)
	assert.Equal(t, clusterName, claim.OwnerReferences[0].Name)
	require.Len(t, pool.GetOwnerReferences(), 1)
	assert.Equal(t, "Cluster", pool.GetOwnerReferences()[0].Kind)
}

// Once the provider has allocated an address the hook records it where
// kube-vip, the CNI and the CCM read it from, and lets creation proceed.
func TestBeforeClusterCreateRecordsTheReservedAddress(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "", &carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"})
	objects := append([]ctrlclient.Object{cluster}, allocatedClaim("10.0.0.42")...)
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objects...).Build()

	handler := New(c)

	// The pass that writes the variable retries, because this reconcile already
	// read the cluster without it.
	first := &runtimehooksv1.BeforeClusterCreateResponse{}
	handler.BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, first)
	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, first.Status)
	assert.Equal(t, int32(retryAfterSeconds), first.RetryAfterSeconds)
	assert.Equal(t, "10.0.0.42", configuredHost(t, c))

	// The next pass finds it recorded and lets creation proceed.
	second := &runtimehooksv1.BeforeClusterCreateResponse{}
	handler.BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, second)
	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, second.Status, second.Message)
	assert.Equal(t, "10.0.0.42", configuredHost(t, c))
}

// Reaching Success takes several passes, so the hook has to be idempotent: a
// second pass must not reserve or record a second address.
func TestBeforeClusterCreateIsIdempotent(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "", &carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"})
	objects := append([]ctrlclient.Object{cluster}, allocatedClaim("10.0.0.42")...)
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objects...).Build()
	handler := New(c)

	for pass := range 4 {
		resp := &runtimehooksv1.BeforeClusterCreateResponse{}
		handler.BeforeClusterCreate(context.Background(),
			&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, resp)
		if pass == 0 {
			// The first pass records the address and asks to be called again.
			require.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status, resp.Message)
			continue
		}
		require.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, "pass %d: %s", pass, resp.Message)
	}

	assert.Equal(t, "10.0.0.42", configuredHost(t, c))

	claims := &ipamv1.IPAddressClaimList{}
	require.NoError(t, c.List(context.Background(), claims))
	assert.Len(t, claims.Items, 1, "a second claim would reserve a second address")
}

// The external endpoint shape: host is an FQDN or floating address the operator
// chose, which kube-vip cannot raise. The reserved address has to become the
// virtual IP, or kube-vip would be templated with the FQDN and the control
// plane would never come up.
func TestBeforeClusterCreateRecordsAVirtualIPBehindAnExternalEndpoint(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "api.example.com", &carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"})
	objects := append([]ctrlclient.Object{cluster}, allocatedClaim("10.0.0.42")...)
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objects...).Build()

	handler := New(c)
	first := &runtimehooksv1.BeforeClusterCreateResponse{}
	handler.BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, first)
	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, first.Status, "the write pass must retry")

	second := &runtimehooksv1.BeforeClusterCreateResponse{}
	handler.BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, second)
	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, second.Status, second.Message)
	assert.Equal(t, "api.example.com", configuredHost(t, c), "the operator's endpoint was overwritten")
	assert.Equal(t, "10.0.0.42", configuredVirtualIP(t, c))
}

// An address the operator already chose for both fields is left entirely alone.
func TestBeforeClusterCreateKeepsAFullyConfiguredEndpoint(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "api.example.com", &carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"})
	clusterConfig, err := apivariables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	require.NoError(t, err)
	clusterConfig.Nutanix.ControlPlaneEndpoint.VirtualIPSpec.Configuration.Address = "10.0.0.5"
	variable, err := apivariables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfig)
	require.NoError(t, err)
	cluster.Spec.Topology.Variables = []clusterv1.ClusterVariable{*variable}

	objects := append([]ctrlclient.Object{cluster}, allocatedClaim("10.0.0.42")...)
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objects...).Build()

	resp := &runtimehooksv1.BeforeClusterCreateResponse{}
	New(c).BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)
	assert.Equal(t, "api.example.com", configuredHost(t, c))
	assert.Equal(t, "10.0.0.5", configuredVirtualIP(t, c))
}

// Prism will reserve its own address if it is inside the pool, and a cluster
// that took it would break the endpoint it manages itself.
func TestBeforeClusterCreateRefusesThePrismCentralAddress(t *testing.T) {
	t.Parallel()

	nutanix := &carenv1.NutanixSpec{
		PrismCentralEndpoint: carenv1.NutanixPrismCentralEndpointSpec{
			URL: "https://10.0.0.42:9440",
			Credentials: carenv1.NutanixPrismCentralEndpointCredentials{
				SecretRef: carenv1.LocalObjectReference{Name: "creds"},
			},
		},
		ControlPlaneEndpoint: carenv1.ControlPlaneEndpointSpec{
			Port: 6443,
			VirtualIPSpec: &carenv1.ControlPlaneVirtualIPSpec{
				Configuration: &carenv1.ControlPlaneVirtualIPConfiguration{
					IPReservation: &carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"},
				},
			},
		},
	}
	variable, err := apivariables.MarshalToClusterVariable(
		carenv1.ClusterConfigVariableName,
		&apivariables.ClusterConfigSpec{Nutanix: nutanix},
	)
	require.NoError(t, err)
	cluster := testCluster(t, "", &carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"})
	cluster.Spec.Topology.Variables = []clusterv1.ClusterVariable{*variable}

	objects := append([]ctrlclient.Object{cluster}, allocatedClaim("10.0.0.42")...)
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objects...).Build()

	resp := &runtimehooksv1.BeforeClusterCreateResponse{}
	New(c).BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
	assert.Contains(t, resp.Message, "is the Prism Central address")
	assert.Empty(t, configuredHost(t, c), "the Prism Central address was recorded")
}

// A claim that points at an address that is not readable yet is a retry, not a
// failure, and must never be recorded.
func TestBeforeClusterCreateWaitsForAnEmptyAddress(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "", &carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"})
	objects := append([]ctrlclient.Object{cluster}, allocatedClaim("")...)
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objects...).Build()

	resp := &runtimehooksv1.BeforeClusterCreateResponse{}
	New(c).BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
	assert.Equal(t, int32(retryAfterSeconds), resp.RetryAfterSeconds)
	assert.Empty(t, configuredHost(t, c))
}

// A value that is not an IP address must not be templated into kube-vip.
func TestBeforeClusterCreateRefusesANonAddress(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "", &carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"})
	objects := append([]ctrlclient.Object{cluster}, allocatedClaim("not-an-ip")...)
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objects...).Build()

	resp := &runtimehooksv1.BeforeClusterCreateResponse{}
	New(c).BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
	assert.Contains(t, resp.Message, "not an IP address")
	assert.Empty(t, configuredHost(t, c))
}

func TestNewNutanixIPPool(t *testing.T) {
	t.Parallel()

	endpoint := &carenv1.NutanixPrismCentralEndpointSpec{
		URL: "https://prism.example.com:9441",
		Credentials: carenv1.NutanixPrismCentralEndpointCredentials{
			SecretRef: carenv1.LocalObjectReference{Name: "creds"},
		},
		AdditionalTrustBundle: "dGVzdA==",
	}

	t.Run("a subnet UUID needs no cluster", func(t *testing.T) {
		t.Parallel()
		pool, err := newNutanixIPPool("p", "ns", endpoint,
			&carenv1.ControlPlaneIPReservation{Subnet: "subnet-uuid"})
		require.NoError(t, err)
		spec, _, _ := unstructured.NestedMap(pool.Object, "spec")
		assert.Equal(t, "subnet-uuid", spec["subnet"])
		assert.NotContains(t, spec, "cluster")
		prismCentral, _, _ := unstructured.NestedMap(pool.Object, "spec", "prismCentral")
		assert.Equal(t, "prism.example.com", prismCentral["address"])
		assert.Equal(t, int64(9441), prismCentral["port"])
		assert.Equal(t, false, prismCentral["insecure"])
		// The provider's field is []byte, which is base64 in JSON, and the value
		// is already base64, so it is passed straight through.
		bundle, _, _ := unstructured.NestedString(pool.Object,
			"spec", "prismCentral", "additionalTrustBundle", "trustBundleData")
		assert.Equal(t, "dGVzdA==", bundle)
	})

	t.Run("a missing endpoint is an error", func(t *testing.T) {
		t.Parallel()
		_, err := newNutanixIPPool("p", "ns", nil, &carenv1.ControlPlaneIPReservation{Subnet: "s"})
		require.Error(t, err)
	})

	t.Run("a missing subnet is an error", func(t *testing.T) {
		t.Parallel()
		_, err := newNutanixIPPool("p", "ns", endpoint, &carenv1.ControlPlaneIPReservation{})
		require.Error(t, err)
	})

	t.Run("a missing credentials secret is an error", func(t *testing.T) {
		t.Parallel()
		_, err := newNutanixIPPool("p", "ns",
			&carenv1.NutanixPrismCentralEndpointSpec{URL: "https://prism.example.com:9440"},
			&carenv1.ControlPlaneIPReservation{Subnet: "s"})
		require.Error(t, err)
	})
}

func TestPoolNameIsStable(t *testing.T) {
	t.Parallel()
	// The name is what makes a retry adopt the previous attempt's claim instead
	// of reserving a second address.
	assert.Equal(t, "workload-control-plane-endpoint", poolName("workload"))
	assert.Equal(t, poolName("workload"), poolName("workload"))
}

// With the Nutanix IPAM provider absent, NutanixIPPool does not exist as a
// kind. Retrying can never fix that, and this hook runs in the bootstrap
// cluster during a self-managed create, so a retry loop would hang the create
// with nothing to point at. It must fail once, with the cause named.
func TestBeforeClusterCreateFailsFastWithoutTheIPAMProvider(t *testing.T) {
	t.Parallel()

	cluster := testCluster(t, "", &carenv1.ControlPlaneIPReservation{Subnet: "vlan170"})
	// This is the error the API machinery returns when a kind has no CRD, which
	// is what a cluster without the provider installed answers with. The fake
	// client tolerates unknown kinds, so it has to be injected.
	noKind := &meta.NoKindMatchError{
		GroupKind:        nutanixIPPoolGVK.GroupKind(),
		SearchedVersions: []string{nutanixIPPoolGVK.Version},
	}
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(cluster).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(
				_ context.Context, _ ctrlclient.WithWatch, obj ctrlclient.Object,
				_ ...ctrlclient.CreateOption,
			) error {
				if obj.GetObjectKind().GroupVersionKind() == nutanixIPPoolGVK {
					return noKind
				}
				return nil
			},
		}).
		Build()

	resp := &runtimehooksv1.BeforeClusterCreateResponse{}
	New(c).BeforeClusterCreate(context.Background(),
		&runtimehooksv1.BeforeClusterCreateRequest{Cluster: *cluster}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds, "a missing provider must not be retried")
	assert.Contains(t, resp.Message, "Nutanix IPAM provider is not installed")
	assert.Empty(t, configuredHost(t, c))
}

func TestPendingOnClaim(t *testing.T) {
	t.Parallel()

	claim := func(conds ...metav1.Condition) *ipamv1.IPAddressClaim {
		return &ipamv1.IPAddressClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "wl-control-plane-endpoint", Namespace: "default"},
			Status:     ipamv1.IPAddressClaimStatus{Conditions: conds},
		}
	}

	t.Run("still pending without a condition, and says where to look", func(t *testing.T) {
		t.Parallel()
		err := pendingOnClaim(claim())
		require.ErrorIs(t, err, errReservationPending)
		assert.Contains(t, err.Error(), "default/wl-control-plane-endpoint")
		assert.Contains(t, err.Error(), "IPAM provider logs")
	})

	t.Run("a failing condition is surfaced instead of swallowed", func(t *testing.T) {
		t.Parallel()
		err := pendingOnClaim(claim(metav1.Condition{
			Type:    "Ready",
			Status:  metav1.ConditionFalse,
			Reason:  "ReservationFailed",
			Message: "kAddressPoolExhausted: Could not get free IP address",
		}))
		require.ErrorIs(t, err, errReservationPending)
		assert.Contains(t, err.Error(), "ReservationFailed")
		assert.Contains(t, err.Error(), "kAddressPoolExhausted")
	})

	t.Run("a satisfied condition is not mistaken for a failure", func(t *testing.T) {
		t.Parallel()
		err := pendingOnClaim(claim(metav1.Condition{
			Type: "Ready", Status: metav1.ConditionTrue, Reason: "Allocated", Message: "done",
		}))
		require.ErrorIs(t, err, errReservationPending)
		assert.NotContains(t, err.Error(), "done")
	})
}
