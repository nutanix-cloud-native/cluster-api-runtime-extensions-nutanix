// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	remotefake "sigs.k8s.io/cluster-api/controllers/remote/fake"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	clusterAUID = "0f1b17bc-289f-4cc2-b558-6eef10065e57"
	clusterBUID = "7c2d1e40-5f3a-4b21-9d8e-11aa22bb33cc"
)

// fakeReleaser records what the sweep asked Prism Central to do.
type fakeReleaser struct {
	subnetExtID string
	reserved    []ReservedIP

	resolveErr   error
	reservedErr  error
	unreserveErr map[string]error

	resolveCalls    []string
	unreserveCalled []string
}

func (f *fakeReleaser) ResolveSubnet(_ context.Context, subnetExtIDOrName, cluster string) (string, error) {
	f.resolveCalls = append(f.resolveCalls, subnetExtIDOrName+"|"+cluster)
	if f.resolveErr != nil {
		return "", f.resolveErr
	}
	return f.subnetExtID, nil
}

func (f *fakeReleaser) ReservedIPs(_ context.Context, _ string) ([]ReservedIP, error) {
	if f.reservedErr != nil {
		return nil, f.reservedErr
	}
	return f.reserved, nil
}

func (f *fakeReleaser) UnreserveContext(_ context.Context, _, clientContext string) error {
	f.unreserveCalled = append(f.unreserveCalled, clientContext)
	return f.unreserveErr[clientContext]
}

func testClusterWithUID(uid string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workload",
			Namespace: "default",
			UID:       types.UID(uid),
		},
	}
}

// ctxA is the reservation context a Service of cluster A carries.
func ctxA(serviceUID string) string { return clusterReservationPrefix(clusterAUID) + serviceUID }

func TestReservationContextsForCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		reserved []ReservedIP
		expect   []string
	}{
		{
			name:     "no reservations",
			reserved: nil,
			expect:   []string{},
		},
		{
			name: "only this cluster's contexts",
			reserved: []ReservedIP{
				{Address: "10.0.0.2", ClientContext: ctxA("svcB")},
				{Address: "10.0.0.1", ClientContext: ctxA("svcA")},
			},
			expect: []string{ctxA("svcA"), ctxA("svcB")},
		},
		{
			// The API server VIP shares the cluster prefix, so one sweep covers
			// both halves of automatic addressing.
			name: "the control-plane VIP is included",
			reserved: []ReservedIP{
				{Address: "10.0.0.1", ClientContext: clusterReservationPrefix(clusterAUID) + "cp"},
			},
			expect: []string{clusterReservationPrefix(clusterAUID) + "cp"},
		},
		{
			name: "another cluster is never touched",
			reserved: []ReservedIP{
				{Address: "10.0.0.1", ClientContext: ctxA("svcA")},
				{Address: "10.0.0.2", ClientContext: clusterReservationPrefix(clusterBUID) + "svcA"},
			},
			expect: []string{ctxA("svcA")},
		},
		{
			// The subnet is shared with VMs and anything else that reserves on it.
			name: "a reservation made by anything else is never touched",
			reserved: []ReservedIP{
				{Address: "10.0.0.1", ClientContext: "some-vm"},
				{Address: "10.0.0.2", ClientContext: ""},
				{Address: "10.0.0.3", ClientContext: "terraform-managed"},
				{Address: "10.0.0.4", ClientContext: "NKP:" + clusterAUID},
			},
			expect: []string{},
		},
		{
			// A past leak can leave two addresses under one context. Unreserve by
			// context releases both, so the context is listed once.
			name: "two addresses under one context collapse to one call",
			reserved: []ReservedIP{
				{Address: "10.0.0.1", ClientContext: ctxA("svcA")},
				{Address: "10.0.0.2", ClientContext: ctxA("svcA")},
			},
			expect: []string{ctxA("svcA")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expect, reservationContextsForCluster(tt.reserved, clusterAUID))
		})
	}
}

func TestReleaseClusterReservations(t *testing.T) {
	t.Parallel()

	t.Run("releases this cluster's reservations and nothing else", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{
			subnetExtID: "subnet-uuid",
			reserved: []ReservedIP{
				{Address: "10.0.0.1", ClientContext: ctxA("svcA")},
				{Address: "10.0.0.2", ClientContext: clusterReservationPrefix(clusterBUID) + "svcA"},
				{Address: "10.0.0.3", ClientContext: "some-vm"},
				{Address: "10.0.0.4", ClientContext: clusterReservationPrefix(clusterAUID) + "cp"},
			},
		}
		err := releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(clusterAUID), "vlan170", "pe-1", logr.Discard())
		require.NoError(t, err)
		assert.Equal(t,
			[]string{clusterReservationPrefix(clusterAUID) + "cp", ctxA("svcA")},
			releaser.unreserveCalled)
		assert.Equal(t, []string{"vlan170|pe-1"}, releaser.resolveCalls)
	})

	t.Run("nothing reserved is a no-op", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{subnetExtID: "subnet-uuid"}
		err := releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(clusterAUID), "subnet-uuid", "", logr.Discard())
		require.NoError(t, err)
		assert.Empty(t, releaser.unreserveCalled)
	})

	t.Run("a second run finds nothing and succeeds", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{
			subnetExtID: "subnet-uuid",
			reserved:    []ReservedIP{{Address: "10.0.0.1", ClientContext: ctxA("svcA")}},
		}
		cluster := testClusterWithUID(clusterAUID)
		require.NoError(t, releaseClusterReservations(context.Background(), releaser,
			cluster, "subnet-uuid", "", logr.Discard()))
		releaser.reserved = nil
		require.NoError(t, releaseClusterReservations(context.Background(), releaser,
			cluster, "subnet-uuid", "", logr.Discard()))
		assert.Equal(t, []string{ctxA("svcA")}, releaser.unreserveCalled)
	})

	t.Run("one unreleasable reservation does not hide the rest", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{
			subnetExtID: "subnet-uuid",
			reserved: []ReservedIP{
				{Address: "10.0.0.1", ClientContext: ctxA("svcA")},
				{Address: "10.0.0.2", ClientContext: ctxA("svcB")},
			},
			unreserveErr: map[string]error{ctxA("svcA"): errors.New("prism unavailable")},
		}
		err := releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(clusterAUID), "subnet-uuid", "", logr.Discard())
		require.ErrorIs(t, err, ErrFailedToReleaseReservations)
		assert.Contains(t, err.Error(), ctxA("svcA"))
		assert.NotContains(t, err.Error(), ctxA("svcB"))
		// Both were attempted: the failure must not stop the sweep.
		assert.Equal(t, []string{ctxA("svcA"), ctxA("svcB")}, releaser.unreserveCalled)
	})

	t.Run("a cluster with no UID releases nothing", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{
			subnetExtID: "subnet-uuid",
			reserved:    []ReservedIP{{Address: "10.0.0.1", ClientContext: ctxA("svcA")}},
		}
		err := releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(""), "subnet-uuid", "", logr.Discard())
		require.ErrorIs(t, err, ErrFailedToReleaseReservations)
		assert.Empty(t, releaser.unreserveCalled)
		assert.Empty(t, releaser.resolveCalls)
	})

	t.Run("an unresolvable subnet releases nothing", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{resolveErr: errors.New(`no subnet named "vlan170"`)}
		err := releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(clusterAUID), "vlan170", "", logr.Discard())
		require.ErrorIs(t, err, ErrFailedToReleaseReservations)
		assert.Empty(t, releaser.unreserveCalled)
	})

	t.Run("a failed listing releases nothing", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{subnetExtID: "subnet-uuid", reservedErr: errors.New("prism unavailable")}
		err := releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(clusterAUID), "subnet-uuid", "", logr.Discard())
		require.ErrorIs(t, err, ErrFailedToReleaseReservations)
		assert.Empty(t, releaser.unreserveCalled)
	})
}

// The prefix must agree with what the CCM writes, or the sweep silently
// releases nothing. This pins the grammar on the CAREN side.
func TestClusterReservationPrefixContract(t *testing.T) {
	t.Parallel()

	prefix := clusterReservationPrefix("0f1b17bc-289f-4cc2-b558-6eef10065e57")
	assert.Equal(t, "nkp:DxsXvCifTMK1WG7vEAZeVw:", prefix)
	assert.Len(t, prefix, 27)
	// A Service context is 49 characters and the VIP's is 29; Prism allows 64.
	assert.Len(t, prefix+"AhNLPy2Dx2eNUXV69XXn4w", 49)
	assert.Len(t, prefix+"cp", 29)
}

// autoIPAMCluster is a cluster whose Service addresses are reserved on Prism
// Central, carried in the topology variable the way CAPI stores it.
func autoIPAMCluster(t *testing.T, uid, subnet string, peCluster *string) *clusterv1.Cluster {
	t.Helper()

	spec := &apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			GenericAddons: carenv1.GenericAddons{
				ServiceLoadBalancer: &carenv1.ServiceLoadBalancer{
					Provider: carenv1.ServiceLoadBalancerProviderMetalLB,
					Configuration: &carenv1.ServiceLoadBalancerConfiguration{
						AutoIPAM: &carenv1.ServiceLoadBalancerAutoIPAM{
							Subnet:  subnet,
							Cluster: peCluster,
						},
					},
				},
			},
		},
		Nutanix: &carenv1.NutanixSpec{
			PrismCentralEndpoint: carenv1.NutanixPrismCentralEndpointSpec{
				URL: "https://prism-central.nutanix.com:9440",
				Credentials: carenv1.NutanixPrismCentralEndpointCredentials{
					SecretRef: carenv1.LocalObjectReference{Name: "creds"},
				},
			},
		},
	}
	variable, err := apivariables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, spec)
	require.NoError(t, err)

	cluster := testClusterWithUID(uid)
	cluster.Spec.Topology = clusterv1.Topology{Variables: []clusterv1.ClusterVariable{*variable}}
	return cluster
}

func TestAutoIPAMReservationSource(t *testing.T) {
	t.Parallel()

	t.Run("automatic mode reports the subnet and endpoint", func(t *testing.T) {
		t.Parallel()
		subnet, peCluster, endpoint, err := autoIPAMReservationSource(
			autoIPAMCluster(t, clusterAUID, "vlan170", ptr.To("pe-1")))
		require.NoError(t, err)
		assert.Equal(t, "vlan170", subnet)
		assert.Equal(t, "pe-1", peCluster)
		require.NotNil(t, endpoint)
		assert.Equal(t, "creds", endpoint.Credentials.SecretRef.Name)
	})

	t.Run("a subnet UUID needs no Prism Element cluster", func(t *testing.T) {
		t.Parallel()
		subnet, peCluster, _, err := autoIPAMReservationSource(
			autoIPAMCluster(t, clusterAUID, "subnet-uuid", nil))
		require.NoError(t, err)
		assert.Equal(t, "subnet-uuid", subnet)
		assert.Empty(t, peCluster)
	})

	t.Run("a cluster with no topology variables reserves nothing", func(t *testing.T) {
		t.Parallel()
		subnet, _, _, err := autoIPAMReservationSource(testClusterWithUID(clusterAUID))
		require.NoError(t, err)
		assert.Empty(t, subnet)
	})

	t.Run("static address ranges reserve nothing", func(t *testing.T) {
		t.Parallel()
		spec := &apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				GenericAddons: carenv1.GenericAddons{
					ServiceLoadBalancer: &carenv1.ServiceLoadBalancer{
						Provider: carenv1.ServiceLoadBalancerProviderMetalLB,
						Configuration: &carenv1.ServiceLoadBalancerConfiguration{
							AddressRanges: []carenv1.AddressRange{{Start: "10.0.0.10", End: "10.0.0.20"}},
						},
					},
				},
			},
			Nutanix: &carenv1.NutanixSpec{},
		}
		variable, err := apivariables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, spec)
		require.NoError(t, err)
		cluster := testClusterWithUID(clusterAUID)
		cluster.Spec.Topology = clusterv1.Topology{Variables: []clusterv1.ClusterVariable{*variable}}

		subnet, _, _, err := autoIPAMReservationSource(cluster)
		require.NoError(t, err)
		assert.Empty(t, subnet)
	})
}

// An opt-out must stop the Prism Central sweep too: it is the only signal an
// operator has for "do not clean up my load balancer resources".
func TestLoadBalancerGCEnabled(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"":      true, // no annotation
		"true":  true,
		"false": false,
		"0":     false,
		"1":     true,
		"junk":  true, // unparseable defaults to opted in, as before
	}
	for value, expect := range tests {
		t.Run("annotation="+value, func(t *testing.T) {
			t.Parallel()
			cluster := testClusterWithUID(clusterAUID)
			if value != "" {
				cluster.Annotations = map[string]string{LoadBalancerGCAnnotation: value}
			}
			assert.Equal(t, expect, loadBalancerGCEnabled(cluster))
		})
	}
}

// The whole point of the sweep: an unreachable control plane means no Service
// can be deleted, so the CCM is never asked to release anything. Before this
// the hook returned Success and every reserved address leaked.
func TestBeforeClusterDeleteSweepsWhenControlPlaneIsUnreachable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		annotations   map[string]string
		reserved      []ReservedIP
		wantStatus    runtimehooksv1.ResponseStatus
		wantUnreserve []string
	}{
		{
			name:          "an unreachable control plane still releases the reservations",
			reserved:      []ReservedIP{{Address: "10.0.0.1", ClientContext: ctxA("svcA")}},
			wantStatus:    runtimehooksv1.ResponseStatusSuccess,
			wantUnreserve: []string{ctxA("svcA")},
		},
		{
			name: "another cluster's reservations on the same subnet survive",
			reserved: []ReservedIP{
				{Address: "10.0.0.1", ClientContext: clusterReservationPrefix(clusterBUID) + "svcA"},
			},
			wantStatus:    runtimehooksv1.ResponseStatusSuccess,
			wantUnreserve: nil,
		},
		{
			name:          "an explicit opt-out stops the sweep too",
			annotations:   map[string]string{LoadBalancerGCAnnotation: "false"},
			reserved:      []ReservedIP{{Address: "10.0.0.1", ClientContext: ctxA("svcA")}},
			wantStatus:    runtimehooksv1.ResponseStatusSuccess,
			wantUnreserve: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			utilruntime.Must(corev1.AddToScheme(scheme))
			utilruntime.Must(clusterv1.AddToScheme(scheme))

			// No ControlPlaneInitialized condition: the control plane is unreachable,
			// so shouldDeleteServicesWithLoadBalancer is false.
			cluster := autoIPAMCluster(t, clusterAUID, "vlan170", ptr.To("pe-1"))
			cluster.Annotations = tt.annotations
			cluster.Status.Phase = string(clusterv1.ClusterPhaseProvisioned)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
			releaser := &fakeReleaser{subnetExtID: "subnet-uuid", reserved: tt.reserved}
			handler := &ServiceLoadBalancerGC{
				client:              fakeClient,
				clusterClientGetter: remotefake.NewClusterClient,
				releaserGetter: func(context.Context, string, *carenv1.NutanixPrismCentralEndpointSpec) (reservationReleaser, error) {
					return releaser, nil
				},
			}

			resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
			handler.BeforeClusterDelete(context.Background(),
				&runtimehooksv1.BeforeClusterDeleteRequest{Cluster: *cluster}, resp)

			assert.Equal(t, tt.wantStatus, resp.Status, resp.Message)
			assert.Equal(t, tt.wantUnreserve, releaser.unreserveCalled)
		})
	}
}

// A Prism Central that cannot be reached must block deletion and retry, not
// let the cluster go with its addresses still held.
func TestBeforeClusterDeleteRetriesWhenPrismIsUnreachable(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))

	cluster := autoIPAMCluster(t, clusterAUID, "vlan170", nil)
	cluster.Status.Phase = string(clusterv1.ClusterPhaseProvisioned)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()

	handler := &ServiceLoadBalancerGC{
		client:              fakeClient,
		clusterClientGetter: remotefake.NewClusterClient,
		releaserGetter: func(context.Context, string, *carenv1.NutanixPrismCentralEndpointSpec) (reservationReleaser, error) {
			return nil, errors.New("prism central unreachable")
		},
	}

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(),
		&runtimehooksv1.BeforeClusterDeleteRequest{Cluster: *cluster}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
	assert.Greater(t, resp.RetryAfterSeconds, int32(0))
	assert.Contains(t, resp.Message, "prism central unreachable")
}

// A subnet that is gone, or was never IPAM-managed, cannot hold a reservation
// of ours. Treating that as a failure would block deleting the cluster forever.
func TestReleaseClusterReservationsTolerantOfAMissingSubnet(t *testing.T) {
	t.Parallel()

	t.Run("an absent subnet is nothing to release", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{
			resolveErr: fmt.Errorf("%w: no subnet named %q", errNothingToRelease, "vlan170"),
		}
		require.NoError(t, releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(clusterAUID), "vlan170", "", logr.Discard()))
		assert.Empty(t, releaser.unreserveCalled)
	})

	t.Run("an unmanaged subnet is nothing to release", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{
			subnetExtID: "subnet-uuid",
			reservedErr: fmt.Errorf("%w: subnet subnet-uuid: %s", errNothingToRelease,
				"Cannot fetch reserved IP addresses for unmanaged subnets"),
		}
		require.NoError(t, releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(clusterAUID), "subnet-uuid", "", logr.Discard()))
		assert.Empty(t, releaser.unreserveCalled)
	})

	t.Run("an ambiguous name is still an error", func(t *testing.T) {
		t.Parallel()
		releaser := &fakeReleaser{
			resolveErr: errors.New(`subnet name "shared" matches 2 subnets`),
		}
		err := releaseClusterReservations(context.Background(), releaser,
			testClusterWithUID(clusterAUID), "shared", "", logr.Discard())
		require.ErrorIs(t, err, ErrFailedToReleaseReservations)
	})
}

// These are the error strings Prism Central actually returns, so the
// classification is pinned to measured behaviour rather than a guess.
func TestIsUnreservableSubnet(t *testing.T) {
	t.Parallel()

	unreservable := []string{
		"list reserved IPs: Cannot fetch reserved IP addresses for unmanaged subnets.",
		"NETWORKING-10056: bad request",
		"subnet not found",
	}
	for _, message := range unreservable {
		assert.True(t, isUnreservableSubnet(errors.New(message)), message)
	}

	retryable := []string{
		"connection refused",
		"context deadline exceeded",
		"401 Unauthorized",
		"Odata Error: Error while parsing URI",
	}
	for _, message := range retryable {
		assert.False(t, isUnreservableSubnet(errors.New(message)), message)
	}
	assert.False(t, isUnreservableSubnet(nil))
}

// The commonest teardown of all: a cluster whose control plane came up once and
// has since died. ClusterControlPlaneInitializedCondition records that it came
// up and stays true, so the cluster takes the delete-the-Services path, fails
// to reach the workload cluster, and used to return a failure without ever
// releasing anything.
func TestBeforeClusterDeleteSweepsWhenTheWorkloadClusterIsUnreachable(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))

	cluster := autoIPAMCluster(t, clusterAUID, "vlan170", nil)
	cluster.Status.Phase = string(clusterv1.ClusterPhaseProvisioned)
	// The control plane came up at some point, so the GC will try to delete the
	// Services rather than skipping.
	cluster.Status.Conditions = []metav1.Condition{{
		Type:   clusterv1.ClusterControlPlaneInitializedCondition,
		Status: metav1.ConditionTrue,
	}}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	releaser := &fakeReleaser{
		subnetExtID: "subnet-uuid",
		reserved: []ReservedIP{
			{Address: "10.0.0.1", ClientContext: ctxA("svcA")},
			{Address: "10.0.0.2", ClientContext: clusterReservationPrefix(clusterBUID) + "svcA"},
		},
	}
	handler := &ServiceLoadBalancerGC{
		client: fakeClient,
		clusterClientGetter: func(
			context.Context, string, ctrlclient.Client, ctrlclient.ObjectKey,
		) (ctrlclient.Client, error) {
			return nil, errors.New("dial tcp 10.0.0.9:6443: connect: connection refused")
		},
		releaserGetter: func(context.Context, string, *carenv1.NutanixPrismCentralEndpointSpec) (reservationReleaser, error) {
			return releaser, nil
		},
	}

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(),
		&runtimehooksv1.BeforeClusterDeleteRequest{Cluster: *cluster}, resp)

	// The response is unchanged: deletion still blocks on the unreachable
	// cluster exactly as it did before.
	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
	assert.Contains(t, resp.Message, "error creating remote cluster client")
	// But this cluster's reservations are released, and only this cluster's.
	assert.Equal(t, []string{ctxA("svcA")}, releaser.unreserveCalled)
}

// A static-range cluster has nothing reserved, so the unreachable path must not
// build a Prism Central client at all.
func TestUnreachableSweepSkipsStaticClusters(t *testing.T) {
	t.Parallel()

	called := false
	handler := &ServiceLoadBalancerGC{
		releaserGetter: func(context.Context, string, *carenv1.NutanixPrismCentralEndpointSpec) (reservationReleaser, error) {
			called = true
			return nil, nil
		},
	}
	require.NoError(t, handler.sweepReservations(
		context.Background(), testClusterWithUID(clusterAUID), logr.Discard()))
	assert.False(t, called, "a cluster with no autoIPAM contacted Prism Central")
}
