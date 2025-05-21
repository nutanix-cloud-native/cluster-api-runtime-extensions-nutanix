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
	name   string
	checks []Check
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Init(_ context.Context, _ ctrlclient.Client, _ *clusterv1.Cluster) []Check {
	return m.checks
}

type mockDecoder struct {
	err error
}

func (m *mockDecoder) Decode(_ admission.Request, _ runtime.Object) error {
	return m.err
}

func (m *mockDecoder) DecodeRaw(_ runtime.RawExtension, _ runtime.Object) error {
	return m.err
}

func TestHandle(t *testing.T) {
	scheme := runtime.NewScheme()
	err := clusterv1.AddToScheme(scheme)
	require.NoError(t, err)

	tests := []struct {
		name             string
		operation        admissionv1.Operation
		decoder          admission.Decoder
		cluster          *clusterv1.Cluster
		checkers         []Checker
		checks           []Check
		expectedResponse admission.Response
	}{
		{
			name:      "skip delete operations",
			operation: admissionv1.Delete,
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
			name: "handle decoder error",
			decoder: &mockDecoder{
				err: fmt.Errorf("decode error"),
			},
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
					Allowed: false,
					Result: &metav1.Status{
						Code:    http.StatusBadRequest,
						Message: "decode error",
					},
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
			name: "internal error takes precedence in response",
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
								Error:   true,
								Message: "internal error",
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
								Allowed: true,
							}
						},
					},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    http.StatusInternalServerError,
						Reason:  metav1.StatusReasonInternalError,
						Message: "preflight checks failed",
						Details: &metav1.StatusDetails{
							Causes: []metav1.StatusCause{
								{
									Type:    metav1.CauseTypeFieldValueInvalid,
									Message: "check failed",
								},
								{
									Type:    metav1.CauseTypeInternal,
									Message: "internal error",
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
			// Default the decoder.
			decoder := admission.NewDecoder(scheme)
			if tt.decoder != nil {
				decoder = tt.decoder
			}

			handler := New(fake.NewClientBuilder().Build(), decoder, tt.checkers...)

			ctx := context.TODO()

			// Create admission request
			jsonCluster, err := json.Marshal(tt.cluster)
			require.NoError(t, err)

			// Default the operation.
			operation := admissionv1.Create
			if tt.operation != "" {
				operation = tt.operation
			}

			admissionReq := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: operation,
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
					assert.ElementsMatch(t, tt.expectedResponse.Result.Details.Causes, got.Result.Details.Causes)
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
				Code:    http.StatusInternalServerError,
				Reason:  metav1.StatusReasonInternalError,
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
			assert.ElementsMatch(t, expectedResponse.Result.Details.Causes, got.Result.Details.Causes)
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
