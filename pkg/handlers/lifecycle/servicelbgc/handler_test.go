// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	remotefake "sigs.k8s.io/cluster-api/controllers/remote/fake"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// testProvisionedCluster returns a provisioned cluster with initialized control plane; used in all cases
// to drive the handler into the service-deletion switch.
// This returns a new resource on each call to avoid sharing resources between test cases.
func testProvisionedCluster(t *testing.T, clusterName, clusterNamespace string) *clusterv1.Cluster {
	t.Helper()

	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterNamespace,
		},
		Status: clusterv1.ClusterStatus{
			Conditions: []metav1.Condition{{
				Type:   clusterv1.ClusterControlPlaneInitializedCondition,
				Status: metav1.ConditionTrue,
			}},
			Phase: string(clusterv1.ClusterPhaseProvisioned),
		},
	}
}

// testLBService returns a LoadBalancer service with an assigned hostname; needsDelete returns true
// for this service, triggering the deletion path.
// This returns a new resource on each call to avoid sharing resources between test cases.
func testLBService(t *testing.T, clusterNamespace string) *corev1.Service {
	t.Helper()
	// A LoadBalancer service with an assigned hostname; needsDelete returns true
	// for this service, triggering the deletion path.
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-lb-service",
			Namespace: clusterNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{Hostname: "lb-123.example.com"}},
			},
		},
	}
}

//nolint:funlen // Long tests are OK
func TestBeforeClusterDelete(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))

	const (
		clusterName      = "test-cluster"
		clusterNamespace = "default"
	)

	tests := []struct {
		name             string
		services         []ctrlclient.Object
		interceptorFuncs interceptor.Funcs
		wantStatus       runtimehooksv1.ResponseStatus
		wantRetryAfter   bool
	}{{
		// default branch: deleteServicesWithLoadBalancer returns nil.
		name:           "no LB services: success with no retry",
		services:       nil,
		wantStatus:     runtimehooksv1.ResponseStatusSuccess,
		wantRetryAfter: false,
	}, {
		// Per CAPI Runtime SDK RetryResponseObject contract, blocking BeforeClusterDelete
		// responses use ResponseStatusSuccess with non-zero RetryAfterSeconds so CAPI
		// requeues cleanly without error backoff while Services finish deleting.
		name:           "LB service still present: blocking success with retry",
		services:       []ctrlclient.Object{testLBService(t, clusterNamespace)},
		wantStatus:     runtimehooksv1.ResponseStatusSuccess,
		wantRetryAfter: true,
	}, {
		// ErrFailedToDeleteService branch: the Delete call fails with a
		// non-NotFound error. The handler MUST return Failure so CAPI retries.
		name:     "LB service delete error: failure with retry",
		services: []ctrlclient.Object{testLBService(t, clusterNamespace)},
		interceptorFuncs: interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				c ctrlclient.WithWatch,
				obj ctrlclient.Object,
				opts ...ctrlclient.DeleteOption,
			) error {
				if _, ok := obj.(*corev1.Service); ok {
					return fmt.Errorf("injected delete failure")
				}
				return c.Delete(ctx, obj, opts...)
			},
		},
		wantStatus:     runtimehooksv1.ResponseStatusFailure,
		wantRetryAfter: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := append(
				[]ctrlclient.Object{
					testProvisionedCluster(
						t,
						clusterName,
						clusterNamespace,
					),
				},
				tt.services...,
			)
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				WithInterceptorFuncs(tt.interceptorFuncs).
				Build()

			handler := &ServiceLoadBalancerGC{
				client:              fakeClient,
				clusterClientGetter: remotefake.NewClusterClient,
			}

			req := &runtimehooksv1.BeforeClusterDeleteRequest{
				Cluster: clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      clusterName,
						Namespace: clusterNamespace,
					},
				},
			}
			resp := &runtimehooksv1.BeforeClusterDeleteResponse{}

			handler.BeforeClusterDelete(context.Background(), req, resp)

			assert.Equal(t, tt.wantStatus, resp.Status)
			if tt.wantRetryAfter {
				assert.Greater(t, resp.RetryAfterSeconds, int32(0))
			} else {
				assert.Equal(t, int32(0), resp.RetryAfterSeconds)
			}
		})
	}
}
