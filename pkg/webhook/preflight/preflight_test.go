// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type mockChecker struct {
	checks []Check
	err    error
}

func (m *mockChecker) Checks(_ context.Context, _ ctrlclient.Client, _ *clusterv1.Cluster) ([]Check, error) {
	return m.checks, m.err
}

func TestHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	err := clusterv1.AddToScheme(scheme)
	require.NoError(t, err)

	tests := []struct {
		name             string
		cluster          *clusterv1.Cluster
		checkers         []Checker
		checks           []Check
		expectedResponse admission.Response
	}{
		{
			name: "skip delete operations",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "test-provider",
					},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
				},
			},
		},
		{
			name: "allow non topology clusters",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "test-provider",
					},
				},
				Spec: clusterv1.ClusterSpec{},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
				},
			},
		},

		{
			name: "if no checks, then allowed",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "test-provider",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			checkers: []Checker{
				&mockChecker{
					checks: []Check{},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
				},
			},
		},
		{
			name: "if all checks pass, then allowed",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "test-provider",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						func(ctx context.Context) CheckResult {
							return CheckResult{Allowed: true}
						},
						func(ctx context.Context) CheckResult {
							return CheckResult{Allowed: true}
						},
					},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
				},
			},
		},

		{
			name: "if any check fails, then not allowed",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "test-provider",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						func(ctx context.Context) CheckResult {
							return CheckResult{
								Allowed: false,
								Field:   "spec.test",
								Message: "test failed",
							}
						},
					},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    http.StatusForbidden,
						Message: "preflight checks failed",
						Details: &metav1.StatusDetails{
							Causes: []metav1.StatusCause{
								{
									Type:    metav1.CauseTypeFieldValueInvalid,
									Field:   "spec.test",
									Message: "test failed",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "return warnings from checks",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "test-provider",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						func(ctx context.Context) CheckResult {
							return CheckResult{
								Allowed: true,
								Warning: "test warning",
							}
						},
					},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:  true,
					Warnings: []string{"test warning"},
					Result: &metav1.Status{
						Details: &metav1.StatusDetails{},
					},
				},
			},
		},
		{
			name: "run other checks, despite checker initialization error",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Labels: map[string]string{
						clusterv1.ProviderNameLabel: "test-provider",
					},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						func(ctx context.Context) CheckResult {
							return CheckResult{
								Allowed: true,
							}
						},
					},
				},
				&mockChecker{
					checks: []Check{
						func(ctx context.Context) CheckResult {
							return CheckResult{
								Allowed: false,
								Message: "check failed",
							}
						},
					},
				},
				&mockChecker{
					checks: []Check{
						func(ctx context.Context) CheckResult {
							return CheckResult{
								Allowed: false,
								Error:   true,
								Message: "check result error",
							}
						},
					},
				},
				&mockChecker{
					err: fmt.Errorf("checker initialization error"),
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    http.StatusForbidden,
						Message: "preflight checks failed",
						Details: &metav1.StatusDetails{
							Causes: []metav1.StatusCause{
								{
									Type:    metav1.CauseTypeInternal,
									Message: "checker initialization error",
								},
								{
									Type:    metav1.CauseTypeInternal,
									Message: "check result error",
								},
								{
									Type:    metav1.CauseTypeFieldValueInvalid,
									Message: "check failed",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := admission.NewDecoder(scheme)

			handler := New(fake.NewClientBuilder().Build(), decoder, tt.checkers...)

			ctx := context.TODO()

			// Create admission request
			jsonCluster, err := json.Marshal(tt.cluster)
			require.NoError(t, err)

			admissionReq := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: jsonCluster,
					},
				},
			}

			// Handle the request
			got := handler.Handle(ctx, admissionReq)

			// Check response fields
			assert.Equal(t, tt.expectedResponse.Allowed, got.Allowed)
			if tt.expectedResponse.Result != nil {
				assert.Equal(t, tt.expectedResponse.Result.Code, got.Result.Code)
				assert.Equal(t, tt.expectedResponse.Result.Message, got.Result.Message)

				if tt.expectedResponse.Result.Details != nil {
					require.NotNil(t, got.Result.Details)
					assert.Len(t, got.Result.Details.Causes, len(tt.expectedResponse.Result.Details.Causes))

					for i, expectedCause := range tt.expectedResponse.Result.Details.Causes {
						assert.Equal(t, expectedCause.Type, got.Result.Details.Causes[i].Type)
						assert.Equal(t, expectedCause.Field, got.Result.Details.Causes[i].Field)
						assert.Equal(t, expectedCause.Message, got.Result.Details.Causes[i].Message)
					}
				}
			}
			assert.Equal(t, tt.expectedResponse.Warnings, got.Warnings)
		})
	}
}

func TestHandleCancelledContext(t *testing.T) {
	scheme := runtime.NewScheme()
	err := clusterv1.AddToScheme(scheme)
	require.NoError(t, err)
	decoder := admission.NewDecoder(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			Labels: map[string]string{
				clusterv1.ProviderNameLabel: "test-provider",
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{},
		},
	}

	checker := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				select {
				case <-ctx.Done():
					return CheckResult{
						Allowed: false,
						Error:   true,
						Message: "context cancelled",
					}
				case <-time.After(100 * time.Millisecond):
					return CheckResult{Allowed: true}
				}
			},
			func(ctx context.Context) CheckResult {
				select {
				case <-ctx.Done():
					return CheckResult{
						Allowed: false,
						Error:   true,
						Message: "context cancelled",
					}
				case <-time.After(100 * time.Millisecond):
					return CheckResult{Allowed: true}
				}
			},
		},
	}

	checkDelay := 50 * time.Millisecond

	expectedResponse := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Code:    http.StatusForbidden,
				Message: "preflight checks failed",
				Details: &metav1.StatusDetails{
					Causes: []metav1.StatusCause{
						{
							Type:    metav1.CauseTypeInternal,
							Message: "context cancelled",
						},
						{
							Type:    metav1.CauseTypeInternal,
							Message: "context cancelled",
						},
					},
				},
			},
		},
	}

	handler := New(fake.NewClientBuilder().Build(), decoder, checker)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		time.Sleep(checkDelay)
		cancel()
	}()

	// Create admission request
	jsonCluster, err := json.Marshal(cluster)
	require.NoError(t, err)

	admissionReq := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: jsonCluster,
			},
		},
	}

	// Handle the request
	got := handler.Handle(ctx, admissionReq)

	// Check response fields
	assert.Equal(t, expectedResponse.Allowed, got.Allowed)
	if expectedResponse.Result != nil {
		assert.Equal(t, expectedResponse.Result.Code, got.Result.Code)
		assert.Equal(t, expectedResponse.Result.Message, got.Result.Message)

		if expectedResponse.Result.Details != nil {
			require.NotNil(t, got.Result.Details)
			assert.Len(t, got.Result.Details.Causes, len(expectedResponse.Result.Details.Causes))

			for i, expectedCause := range expectedResponse.Result.Details.Causes {
				assert.Equal(t, expectedCause.Type, got.Result.Details.Causes[i].Type)
				assert.Equal(t, expectedCause.Field, got.Result.Details.Causes[i].Field)
				assert.Equal(t, expectedCause.Message, got.Result.Details.Causes[i].Message)
			}
		}
	}
	assert.Equal(t, expectedResponse.Warnings, got.Warnings)
}

func TestHandleParallelChecks(t *testing.T) {
	scheme := runtime.NewScheme()
	err := clusterv1.AddToScheme(scheme)
	require.NoError(t, err)

	decoder := admission.NewDecoder(scheme)

	// Test that checks run in parallel by using atomic counter
	var counter int32
	checker := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				current := atomic.AddInt32(&counter, 1)
				// Sleep to ensure other goroutines can increment counter if running in parallel
				time.Sleep(50 * time.Millisecond)
				if current == 2 {
					return CheckResult{Allowed: true}
				}
				return CheckResult{Allowed: true}
			},
			func(ctx context.Context) CheckResult {
				current := atomic.AddInt32(&counter, 1)
				if current == 2 {
					return CheckResult{Allowed: true}
				}
				return CheckResult{Allowed: true}
			},
		},
	}

	handler := New(fake.NewClientBuilder().Build(), decoder, checker)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			Labels: map[string]string{
				clusterv1.ProviderNameLabel: "test-provider",
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{},
		},
	}

	jsonCluster, err := json.Marshal(cluster)
	require.NoError(t, err)

	admissionReq := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: jsonCluster,
			},
		},
	}

	got := handler.Handle(context.Background(), admissionReq)
	assert.True(t, got.Allowed)

	// If counter reached 2 before the first check finished its sleep,
	// it means the checks ran in parallel
	assert.Equal(t, int32(2), atomic.LoadInt32(&counter))
}
