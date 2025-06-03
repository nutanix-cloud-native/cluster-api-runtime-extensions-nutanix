// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
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

func mockCheckerFactory(checker Checker) CheckerFactory {
	return func(_ ctrlclient.Client, _ *clusterv1.Cluster) Checker {
		return checker
	}
}

type mockChecker struct {
	checks []Check
}

func (m *mockChecker) Init(_ context.Context) []Check {
	return m.checks
}

type mockDecoder struct {
	err error
}

//nolint:gocritic // These parameters are required, because this mock implements a third-party interface.
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
								Name:    "Test1",
								Allowed: false,
								Causes: []Cause{
									{
										Field:   "spec.test",
										Message: "test failed",
									},
								},
							}
						},
					},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    http.StatusUnprocessableEntity,
						Reason:  metav1.StatusReasonInvalid,
						Message: "preflight checks failed",
						Details: &metav1.StatusDetails{
							Causes: []metav1.StatusCause{
								{
									Type:    "FailedPreflightTest1",
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
								Warnings: []string{
									"test warning",
								},
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
								Name:    "Test1",
								Allowed: false,
								Error:   true,

								Causes: []Cause{
									{
										Message: "internal error",
									},
								},
							}
						},
					},
				},
				&mockChecker{
					checks: []Check{
						func(ctx context.Context) CheckResult {
							return CheckResult{
								Name:    "Test2",
								Allowed: false,
								Causes: []Cause{
									{
										Message: "check failed",
									},
								},
							}
						},
					},
				},
				&mockChecker{
					checks: []Check{
						func(ctx context.Context) CheckResult {
							return CheckResult{
								Name:    "Test3",
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
						Message: "preflight checks failed due to an internal error",
						Details: &metav1.StatusDetails{
							Causes: []metav1.StatusCause{
								{
									Type:    "FailedPreflightTest2",
									Message: "check failed",
								},
								{
									Type:    "FailedPreflightTest1",
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

			factories := make([]CheckerFactory, len(tt.checkers))
			for i, checker := range tt.checkers {
				factories[i] = mockCheckerFactory(checker)
			}
			handler := New(fake.NewClientBuilder().Build(), decoder, factories...)

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
						Name:    "Test1",
						Allowed: false,
						Error:   true,
						Causes: []Cause{
							{
								Message: "context cancelled",
							},
						},
					}
				case <-time.After(100 * time.Millisecond):
					return CheckResult{Allowed: true}
				}
			},
			func(ctx context.Context) CheckResult {
				select {
				case <-ctx.Done():
					return CheckResult{
						Name:    "Test2",
						Allowed: false,
						Error:   true,
						Causes: []Cause{
							{
								Message: "context cancelled",
							},
						},
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
				Message: "preflight checks failed due to an internal error",
				Details: &metav1.StatusDetails{
					Causes: []metav1.StatusCause{
						{
							Type:    "FailedPreflightTest1",
							Message: "context cancelled",
						},
						{
							Type:    "FailedPreflightTest2",
							Message: "context cancelled",
						},
					},
				},
			},
		},
	}

	handler := New(fake.NewClientBuilder().Build(), decoder, mockCheckerFactory(checker))

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

func TestRun_NoCheckers(t *testing.T) {
	ctx := context.Background()
	results := run(ctx, nil, nil, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRun_SingleCheckerSingleCheck(t *testing.T) {
	ctx := context.Background()
	checker := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				return CheckResult{Name: "check1", Allowed: true}
			},
		},
	}
	factory := mockCheckerFactory(checker)
	resultsOrderedByCheckerAndCheck := run(ctx, nil, nil, []CheckerFactory{factory})
	if len(resultsOrderedByCheckerAndCheck) != 1 {
		t.Fatalf("expected results for 1 checker, got %d", len(resultsOrderedByCheckerAndCheck))
	}
	results := resultsOrderedByCheckerAndCheck[0]
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "check1" || !results[0].Allowed {
		t.Errorf("unexpected result: %+v", results[0])
	}
}

func TestRun_MultipleCheckersMultipleChecks(t *testing.T) {
	ctx := context.Background()
	checker1 := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				return CheckResult{Name: "c1-check1", Allowed: true}
			},
			func(ctx context.Context) CheckResult {
				return CheckResult{Name: "c1-check2", Allowed: false}
			},
		},
	}
	checker2 := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				return CheckResult{Name: "c2-check1", Error: true}
			},
		},
	}
	factories := []CheckerFactory{
		mockCheckerFactory(checker1),
		mockCheckerFactory(checker2),
	}
	resultsOrderedByCheckerAndCheck := run(ctx, nil, nil, factories)
	if len(resultsOrderedByCheckerAndCheck) != 2 {
		t.Fatalf("expected results for 2 checkers, got %d", len(resultsOrderedByCheckerAndCheck))
	}

	expected := []string{"c1-check1", "c1-check2", "c2-check1"}
	expectedIdx := 0
	for _, results := range resultsOrderedByCheckerAndCheck {
		for _, result := range results {
			if result.Name != expected[expectedIdx] {
				t.Errorf("expected result %d to be %q, got %q", expectedIdx, expected[expectedIdx], result.Name)
			}
			expectedIdx++
		}
	}
}

func TestRun_ChecksRunInParallel(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	order := []string{}
	checker := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				time.Sleep(50 * time.Millisecond)
				mu.Lock()
				order = append(order, "slow")
				mu.Unlock()
				return CheckResult{Name: "slow"}
			},
			func(ctx context.Context) CheckResult {
				mu.Lock()
				order = append(order, "fast")
				mu.Unlock()
				return CheckResult{Name: "fast"}
			},
		},
	}
	factory := mockCheckerFactory(checker)
	resultsOrderedByCheckerAndCheck := run(ctx, nil, nil, []CheckerFactory{factory})

	results := resultsOrderedByCheckerAndCheck[0]
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Both checks should have run, order in 'order' should be "fast", "slow" if parallel.
	if order[0] != "fast" || order[1] != "slow" {
		t.Errorf("expected order [fast slow], got %v", order)
	}
}

func TestRun_CheckersRunInParallel(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	order := []string{}
	checker1 := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				time.Sleep(50 * time.Millisecond)
				mu.Lock()
				order = append(order, "slow-checker")
				mu.Unlock()
				return CheckResult{Name: "slow-checker"}
			},
		},
	}
	checker2 := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				mu.Lock()
				order = append(order, "fast-checker")
				mu.Unlock()
				return CheckResult{Name: "fast-checker"}
			},
		},
	}
	factories := []CheckerFactory{
		mockCheckerFactory(checker1),
		mockCheckerFactory(checker2),
	}
	results := run(ctx, nil, nil, factories)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Both checkers should have run, order in 'order' should be "fast-checker", "slow-checker" if parallel.
	if order[0] != "fast-checker" || order[1] != "slow-checker" {
		t.Errorf("expected order [fast-checker slow-checker], got %v", order)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Use a channel to synchronize and ensure the check started
	started := make(chan struct{})
	completed := make(chan struct{})

	checker := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				close(started)
				select {
				case <-ctx.Done():
					return CheckResult{Name: "cancelled", Error: true}
				case <-time.After(2 * time.Second):
					close(completed)
					return CheckResult{Name: "completed", Allowed: true}
				}
			},
		},
	}

	go func() {
		<-started
		cancel()
	}()

	factory := mockCheckerFactory(checker)
	resultsOrderedByCheckerAndCheck := run(ctx, nil, nil, []CheckerFactory{factory})
	if len(resultsOrderedByCheckerAndCheck) != 1 {
		t.Fatalf("expected results for 1 checker, got %d", len(resultsOrderedByCheckerAndCheck))
	}

	select {
	case <-completed:
		t.Error("check should have been cancelled but completed")
	case <-time.After(50 * time.Millisecond):
		// This is expected - the check was cancelled
	}

	results := resultsOrderedByCheckerAndCheck[0]
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Error {
		t.Errorf("expected error result after cancellation, got: %+v", results[0])
	}
}

func TestRun_OrderOfResults(t *testing.T) {
	ctx := context.Background()

	checker1 := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				time.Sleep(30 * time.Millisecond)
				return CheckResult{Name: "c1-check1"}
			},
			func(ctx context.Context) CheckResult {
				time.Sleep(10 * time.Millisecond)
				return CheckResult{Name: "c1-check2"}
			},
		},
	}

	checker2 := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				return CheckResult{Name: "c2-check1"}
			},
			func(ctx context.Context) CheckResult {
				time.Sleep(20 * time.Millisecond)
				return CheckResult{Name: "c2-check2"}
			},
		},
	}

	factories := []CheckerFactory{
		mockCheckerFactory(checker1),
		mockCheckerFactory(checker2),
	}
	resultsOrderedByCheckerAndCheck := run(ctx, nil, nil, factories)
	if len(resultsOrderedByCheckerAndCheck) != 2 {
		t.Fatalf("expected results for 2 checkers, got %d", len(resultsOrderedByCheckerAndCheck))
	}

	// The order should be all checks from checker1 followed by all checks from checker2,
	// regardless of their completion time
	expected := []string{"c1-check1", "c1-check2", "c2-check1", "c2-check2"}
	expectedIdx := 0
	for _, results := range resultsOrderedByCheckerAndCheck {
		for _, result := range results {
			if result.Name != expected[expectedIdx] {
				t.Errorf("expected result %d to be %q, got %q", expectedIdx, expected[expectedIdx], result.Name)
			}
			expectedIdx++
		}
	}
}

func TestRun_LargeNumberOfCheckersAndChecks(t *testing.T) {
	ctx := context.Background()

	checkerCount := 10
	checksPerChecker := 50

	checkers := make([]Checker, checkerCount)
	expectedTotal := checkerCount * checksPerChecker

	for i := 0; i < checkerCount; i++ {
		checks := make([]Check, checksPerChecker)
		for j := 0; j < checksPerChecker; j++ {
			checkerIndex := i
			checkIndex := j
			checks[j] = func(ctx context.Context) CheckResult {
				return CheckResult{
					Name:    fmt.Sprintf("checker%d-check%d", checkerIndex, checkIndex),
					Allowed: true,
				}
			}
		}
		checkers[i] = &mockChecker{
			checks: checks,
		}
	}

	factories := make([]CheckerFactory, checkerCount)
	for i, checker := range checkers {
		factories[i] = mockCheckerFactory(checker)
	}

	start := time.Now()
	resultsOrderedByCheckerAndCheck := run(ctx, nil, nil, factories)
	duration := time.Since(start)

	resultTotal := 0
	for _, results := range resultsOrderedByCheckerAndCheck {
		resultTotal += len(results)
	}

	if resultTotal != expectedTotal {
		t.Errorf("expected %d results, got %d", expectedTotal, resultTotal)
	}

	t.Logf("Completed %d checks in %v", expectedTotal, duration)
}

func TestRun_ErrorHandlingInChecks(t *testing.T) {
	ctx := context.Background()

	panicCheck := func(ctx context.Context) CheckResult {
		// This function should never panic since we recover in the test,
		// but in real code the goroutine would crash
		panic("simulated error in check")
	}

	// Wrap the check with panic recovery
	safeCheck := func(ctx context.Context) CheckResult {
		defer func() {
			if r := recover(); r != nil { //nolint:staticcheck // This is a test, we want to recover from panic
			}
		}()
		return panicCheck(ctx)
	}

	checker := &mockChecker{
		checks: []Check{
			safeCheck,
			func(ctx context.Context) CheckResult {
				return CheckResult{Name: "normal-check", Allowed: true}
			},
		},
	}

	factory := mockCheckerFactory(checker)
	resultsOrderedByCheckerAndCheck := run(ctx, nil, nil, []CheckerFactory{factory})

	if len(resultsOrderedByCheckerAndCheck) != 1 {
		t.Fatalf("expected results for 1 checker, got %d", len(resultsOrderedByCheckerAndCheck))
	}

	results := resultsOrderedByCheckerAndCheck[0]
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// The normal check should have succeeded
	found := false
	for _, result := range results {
		if result.Name == "normal-check" {
			found = true
			if !result.Allowed {
				t.Errorf("expected normal-check to be allowed")
			}
		}
	}

	if !found {
		t.Errorf("normal-check result not found")
	}
}

func TestRun_ZeroChecksFromChecker(t *testing.T) {
	ctx := context.Background()

	// Checker that returns no checks
	emptyChecker := &mockChecker{
		checks: []Check{},
	}

	// Checker that returns some checks
	normalChecker := &mockChecker{
		checks: []Check{
			func(ctx context.Context) CheckResult {
				return CheckResult{Name: "check1", Allowed: true}
			},
		},
	}

	factories := []CheckerFactory{
		mockCheckerFactory(emptyChecker),
		mockCheckerFactory(normalChecker),
	}
	resultsOrderedByCheckerAndCheck := run(ctx, nil, nil, factories)

	if len(resultsOrderedByCheckerAndCheck) != 2 {
		t.Fatalf("expected results for 2 checkers, got %d", len(resultsOrderedByCheckerAndCheck))
	}

	emptyResults := resultsOrderedByCheckerAndCheck[0] // We expect no results from the empty checker
	if len(emptyResults) != 0 {
		t.Fatalf("expected 0 results from empty checker, got %d", len(emptyResults))
	}

	normalResults := resultsOrderedByCheckerAndCheck[1] // We expect results from the normal checker
	if len(normalResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(normalResults))
	}

	if normalResults[0].Name != "check1" {
		t.Errorf("expected result from normal checker, got %s", normalResults[0].Name)
	}
}
