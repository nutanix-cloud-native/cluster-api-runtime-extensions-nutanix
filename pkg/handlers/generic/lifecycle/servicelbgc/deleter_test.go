// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

//nolint:funlen // Long tests are OK
func Test_shouldDeleteServicesWithLoadBalancer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cluster      *v1beta1.Cluster
		shouldDelete bool
	}{{
		name: "should delete",
		cluster: &v1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-should-delete",
			},
			Status: v1beta1.ClusterStatus{
				Conditions: v1beta1.Conditions{{
					Type:   v1beta1.ControlPlaneInitializedCondition,
					Status: corev1.ConditionTrue,
				}},
				Phase: string(v1beta1.ClusterPhaseProvisioned),
			},
		},
		shouldDelete: true,
	}, {
		name: "should delete: annotation is set to true",
		cluster: &v1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-should-delete",
				Annotations: map[string]string{
					LoadBalancerGCAnnotation: "true",
				},
			},
			Status: v1beta1.ClusterStatus{
				Conditions: v1beta1.Conditions{{
					Type:   v1beta1.ControlPlaneInitializedCondition,
					Status: corev1.ConditionTrue,
				}},
				Phase: string(v1beta1.ClusterPhaseProvisioned),
			},
		},
		shouldDelete: true,
	}, {
		name: "should not delete: annotation is set to false",
		cluster: &v1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-should-delete",
				Annotations: map[string]string{
					LoadBalancerGCAnnotation: "false",
				},
			},
			Status: v1beta1.ClusterStatus{
				Phase: string(v1beta1.ClusterPhaseProvisioned),
			},
		},
		shouldDelete: false,
	}, {
		name: "should not delete: phase is Pending",
		cluster: &v1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-should-delete",
			},
			Status: v1beta1.ClusterStatus{
				Phase: string(v1beta1.ClusterPhasePending),
			},
		},
		shouldDelete: false,
	}, {
		name: "should not delete: ControlPlaneInitialized condition is False",
		cluster: &v1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-should-delete",
			},
			Status: v1beta1.ClusterStatus{
				Conditions: v1beta1.Conditions{{
					Type:   v1beta1.ControlPlaneInitializedCondition,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		shouldDelete: false,
	}}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			shouldDelete, err := shouldDeleteServicesWithLoadBalancer(tt.cluster)
			assert.NoError(t, err)
			assert.Equal(t, tt.shouldDelete, shouldDelete)
		})
	}
}

//nolint:funlen // Long tests are OK
func Test_deleteServicesWithLoadBalancer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		startServices []corev1.Service
		endServices   []corev1.Service
	}{{
		name:          "no services",
		startServices: []corev1.Service(nil),
		endServices:   []corev1.Service(nil),
	}, {
		name: "should not delete, all services with ClusterIP",
		startServices: []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-1",
				Namespace: "ns-1",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-2",
				Namespace: "ns-2",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}},
		endServices: []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "svc-1",
				Namespace:       "ns-1",
				ResourceVersion: "1",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:            "svc-2",
				Namespace:       "ns-2",
				ResourceVersion: "1",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}},
	}, {
		name: "should delete 1 service with LoadBalancer",
		startServices: []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-1",
				Namespace: "ns-1",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-2",
				Namespace: "ns-2",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{Hostname: "lb-123.com"}},
				},
			},
		}},
		endServices: []corev1.Service{{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "svc-1",
				Namespace:       "ns-1",
				ResourceVersion: "1",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}},
	}}

	for idx := range tests {
		tt := tests[idx] // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fakeClient := fake.NewClientBuilder().Build()
			for i := range tt.startServices {
				svc := &tt.startServices[i]
				require.NoError(
					t,
					fakeClient.Create(context.Background(), svc),
					"error creating Service",
				)
			}

			for {
				err := deleteServicesWithLoadBalancer(context.TODO(), fakeClient, logr.Discard())
				if err == nil {
					break
				}
				require.ErrorIs(
					t,
					err,
					ErrServicesStillExist,
					"only allowed a retry-able error here",
				)
			}

			services := &corev1.ServiceList{}
			require.NoError(
				t,
				fakeClient.List(context.Background(), services),
				"error listing Services",
			)
			assert.Equal(t, tt.endServices, services.Items)
		})
	}
}

func Test_needsDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		service      *corev1.Service
		shouldDelete bool
	}{{
		name: "shouldDelete",
		service: &corev1.Service{
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{Hostname: "lb-123.com"}},
				},
			},
		},
		shouldDelete: true,
	}, {
		name: "false: ServiceTypeNodePort",
		service: &corev1.Service{
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{Hostname: "lb-123.com"}},
				},
			},
		},
		shouldDelete: false,
	}, {
		name: "false: LoadBalancer is empty",
		service: &corev1.Service{
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
			},
			Status: corev1.ServiceStatus{},
		},
		shouldDelete: false,
	}}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			del := needsDelete(tt.service)
			assert.Equal(t, tt.shouldDelete, del)
		})
	}
}

// this test is mainly here to visually show what the error will look like.
func Test_failedToDeleteServicesError(t *testing.T) {
	svcs := []client.ObjectKey{
		{Namespace: "ns-1", Name: "svc-1"},
		{Namespace: "ns-2", Name: "svc-2"},
		{Namespace: "ns-3", Name: "svc-3"},
		{Namespace: "ns-4", Name: "svc-4"},
		{Namespace: "ns-5", Name: "svc-5"},
	}
	//nolint:lll // want to show the full error in one line
	expectedErrString := "failed to delete kubernetes services: the following Services could not be deleted and must cleaned up manually before deleting the cluster: ns-1/svc-1, ns-2/svc-2, ns-3/svc-3, ns-4/svc-4, ns-5/svc-5"
	assert.EqualError(t, failedToDeleteServicesError(svcs), expectedErrString)
}
