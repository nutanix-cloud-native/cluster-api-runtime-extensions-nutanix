// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/skip"
)

type mockChecker struct {
	checks []Check
}

func (m *mockChecker) Init(_ context.Context, _ ctrlclient.Client, _ *clusterv1.Cluster) []Check {
	return m.checks
}

type mockCheck struct {
	name   string
	result CheckResult
	run    bool
}

func (m *mockCheck) Name() string {
	return m.name
}

func (m *mockCheck) Run(_ context.Context) CheckResult {
	m.run = true
	return m.result
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

func topologyCluster(skippedCheckNames ...string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
			Annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: strings.Join(skippedCheckNames, ","),
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{},
		},
	}
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
		expectedResponse admission.Response
	}{
		// Keeping the existing test cases that don't use checks
		{
			name:      "skip delete operations",
			operation: admissionv1.Delete,
			cluster:   topologyCluster(),
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
				},
			},
		},
		{
			name: "skip paused clusters",
			cluster: func() *clusterv1.Cluster {
				cluster := topologyCluster()
				cluster.Spec.Paused = true
				return cluster
			}(),
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
			cluster: topologyCluster(),
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
			name:    "if no checks, then allowed",
			cluster: topologyCluster(),
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
			name:    "if cluster skips all checks, then allowed, with a warning",
			cluster: topologyCluster(carenv1.PreflightChecksSkipAllAnnotationValue),
			checkers: []Checker{
				&mockChecker{
					checks: []Check{},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Warnings: []string{
						"Cluster has skipped all preflight checks",
					},
				},
			},
		},
		{
			name:    "if cluster skips a check, then that check is not run",
			cluster: topologyCluster("SkippedCheck"),
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name:   "SkippedCheck",
							result: CheckResult{},
						},
						&mockCheck{
							name: "OtherCheck",
							result: CheckResult{
								Allowed: true,
							},
						},
					},
				},
			},
			expectedResponse: admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Warnings: []string{
						"Cluster has skipped preflight check \"SkippedCheck\"",
					},
				},
			},
		},
		{
			name:    "if all checks pass, then allowed",
			cluster: topologyCluster(),
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name: "Check1",
							result: CheckResult{
								Allowed: true,
							},
						},
						&mockCheck{
							name: "Check2",
							result: CheckResult{
								Allowed: true,
							},
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
			name:    "if any check fails, then not allowed",
			cluster: topologyCluster(),
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name: "Test1",
							result: CheckResult{
								Allowed: false,
								Causes: []Cause{
									{
										Field:   "spec.test",
										Message: "test failed",
									},
								},
							},
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
									Type:    "Test1",
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
			name:    "return warnings from checks",
			cluster: topologyCluster(),
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name: "Test1",
							result: CheckResult{
								Allowed: true,
								Warnings: []string{
									"test warning",
								},
							},
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
			name:    "internal error takes precedence in response",
			cluster: topologyCluster(),
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name: "Test1",
							result: CheckResult{
								Allowed:       false,
								InternalError: true,
								Causes: []Cause{
									{
										Message: "internal error",
									},
								},
							},
						},
					},
				},
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name: "Test2",
							result: CheckResult{
								Allowed: false,
								Causes: []Cause{
									{
										Message: "check failed",
									},
								},
							},
						},
					},
				},
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name: "Test3",
							result: CheckResult{
								Allowed: true,
							},
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
									Type:    "Test2",
									Message: "check failed",
								},
								{
									Type:    "Test1",
									Message: "internal error",
								},
							},
						},
					},
				},
			},
		},
		{
			name:      "update operation with passing checks, allowed",
			operation: admissionv1.Update,
			cluster:   topologyCluster(),
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name: "Test1",
							result: CheckResult{
								Allowed: true,
							},
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
			name:      "update operation with failing checks, not allowed",
			operation: admissionv1.Update,
			cluster:   topologyCluster(),
			checkers: []Checker{
				&mockChecker{
					checks: []Check{
						&mockCheck{
							name: "Test1",
							result: CheckResult{
								Allowed: false,
								Causes: []Cause{
									{
										Message: "check failed",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Test execution remains the same
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

// Define a custom check that can be cancelled.
type cancellableCheck struct {
	name      string
	delay     time.Duration
	cancelled bool
}

// Implement the Check interface.
func (c *cancellableCheck) Name() string {
	return c.name
}

func (c *cancellableCheck) Run(ctx context.Context) CheckResult {
	select {
	case <-ctx.Done():
		c.cancelled = true
		return CheckResult{
			Allowed:       false,
			InternalError: true,
			Causes: []Cause{
				{
					Message: "context cancelled",
				},
			},
		}
	case <-time.After(c.delay):
		return CheckResult{
			Allowed: true,
		}
	}
}

// TestHandleCancelledContext needs special treatment because it relies on context cancellation.
func TestHandleCancelledContext(t *testing.T) {
	scheme := runtime.NewScheme()
	err := clusterv1.AddToScheme(scheme)
	require.NoError(t, err)
	decoder := admission.NewDecoder(scheme)

	cluster := topologyCluster()

	// Create cancellable checks
	check1 := &cancellableCheck{
		name:  "Test1",
		delay: 100 * time.Millisecond,
	}
	check2 := &cancellableCheck{
		name:  "Test2",
		delay: 100 * time.Millisecond,
	}

	checker := &mockChecker{
		checks: []Check{check1, check2},
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
							Type:    "Test1",
							Message: "context cancelled",
						},
						{
							Type:    "Test2",
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

	// Verify checks were cancelled
	assert.True(t, check1.cancelled, "Check1 should have been cancelled")
	assert.True(t, check2.cancelled, "Check2 should have been cancelled")

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
	results := run(ctx, nil, nil, nil, nil)
	assert.Empty(t, results, "expected no results when no checkers are provided")
}

func TestRun_SingleCheckerSingleCheck(t *testing.T) {
	ctx := context.Background()
	cluster := topologyCluster()
	checker := &mockChecker{
		checks: []Check{
			&mockCheck{
				name: "check1",
				result: CheckResult{
					Allowed: true,
				},
			},
		},
	}
	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), []Checker{checker})
	if len(resultsOrderedByCheckerAndCheck) != 1 {
		t.Fatalf("expected results for 1 checker, got %d", len(resultsOrderedByCheckerAndCheck))
	}
	results := resultsOrderedByCheckerAndCheck[0]
	assert.Len(t, results, 1, "expected 1 result from the checker")
	assert.Equal(t, "check1", results[0].Name, "expected result name to be 'check1'")
	assert.True(t, results[0].Allowed, "expected result to be allowed")
}

func TestRun_MultipleCheckersMultipleChecks(t *testing.T) {
	ctx := context.Background()
	cluster := topologyCluster()
	checker1 := &mockChecker{
		checks: []Check{
			&mockCheck{
				name: "c1-check1",
				result: CheckResult{
					Allowed: true,
				},
			},
			&mockCheck{
				name: "c1-check2",
				result: CheckResult{
					Allowed: false,
				},
			},
		},
	}
	checker2 := &mockChecker{
		checks: []Check{
			&mockCheck{
				name: "c2-check1",
				result: CheckResult{
					InternalError: true,
				},
			},
		},
	}

	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), []Checker{checker1, checker2})
	if len(resultsOrderedByCheckerAndCheck) != 2 {
		t.Fatalf("expected results for 2 checkers, got %d", len(resultsOrderedByCheckerAndCheck))
	}

	expected := []string{"c1-check1", "c1-check2", "c2-check1"}
	expectedIdx := 0
	for _, results := range resultsOrderedByCheckerAndCheck {
		for _, result := range results {
			assert.Equal(t, expected[expectedIdx], result.Name, "expected result name to match")
			expectedIdx++
		}
	}
}

// For tests that depend on execution timing, we'll need special implementations.
type delayedCheck struct {
	name  string
	delay time.Duration
	mu    *sync.Mutex
	order *[]string
}

func (c *delayedCheck) Name() string {
	return c.name
}

func (c *delayedCheck) Run(ctx context.Context) CheckResult {
	if c.delay > 0 {
		time.Sleep(c.delay)
	}

	if c.mu != nil && c.order != nil {
		c.mu.Lock()
		*c.order = append(*c.order, c.name)
		c.mu.Unlock()
	}

	return CheckResult{Allowed: true}
}

func TestRun_ChecksRunInParallel(t *testing.T) {
	ctx := context.Background()
	cluster := topologyCluster()

	var mu sync.Mutex
	order := []string{}

	checker := &mockChecker{
		checks: []Check{
			&delayedCheck{
				name:  "slow-check",
				delay: 50 * time.Millisecond,
				mu:    &mu,
				order: &order,
			},
			&delayedCheck{
				name:  "fast-check",
				delay: 0,
				mu:    &mu,
				order: &order,
			},
		},
	}
	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), []Checker{checker})

	results := resultsOrderedByCheckerAndCheck[0]
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Both checks should have run, order in 'order' should be "fast", "slow" if parallel.
	assert.Equal(t, "fast-check", order[0], "expected first recorded result to be 'fast-check'")
	assert.Equal(t, "slow-check", order[1], "expected second recorded result to be 'slow-check'")
}

func TestRun_CheckersRunInParallel(t *testing.T) {
	ctx := context.Background()
	cluster := topologyCluster()

	var mu sync.Mutex
	order := []string{}

	checker1 := &mockChecker{
		checks: []Check{
			&delayedCheck{
				name:  "slow-checker",
				delay: 50 * time.Millisecond,
				mu:    &mu,
				order: &order,
			},
		},
	}
	checker2 := &mockChecker{
		checks: []Check{
			&delayedCheck{
				name:  "fast-checker",
				delay: 0,
				mu:    &mu,
				order: &order,
			},
		},
	}

	results := run(ctx, nil, cluster, skip.New(cluster), []Checker{checker1, checker2})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Both checkers should have run, order in 'order' should be "fast-checker", "slow-checker" if parallel.
	assert.Equal(t, "fast-checker", order[0], "expected first recorded result to be 'fast-checker'")
	assert.Equal(t, "slow-checker", order[1], "expected second recorded result to be 'slow-checker'")
}

type contextAwareCheck struct {
	name      string
	onCancel  func() CheckResult
	onTimeout func() CheckResult
}

func (c *contextAwareCheck) Name() string {
	return c.name
}

func (c *contextAwareCheck) Run(ctx context.Context) CheckResult {
	select {
	case <-ctx.Done():
		return c.onCancel()
	case <-time.After(2 * time.Second):
		return c.onTimeout()
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cluster := topologyCluster()

	// Use channels to synchronize test execution
	started := make(chan struct{})
	completed := make(chan struct{})

	check := &contextAwareCheck{
		name: "cancellable-check",
		onCancel: func() CheckResult {
			return CheckResult{InternalError: true}
		},
		onTimeout: func() CheckResult {
			close(completed)
			return CheckResult{Allowed: true}
		},
	}

	checker := &mockChecker{
		checks: []Check{check},
	}

	// Signal that the check has started and should be cancelled
	go func() {
		close(started)
		cancel()
	}()

	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), []Checker{checker})

	select {
	case <-completed:
		t.Error("check should have been cancelled but completed")
	case <-time.After(50 * time.Millisecond):
		// This is expected - the check was cancelled
	}

	if len(resultsOrderedByCheckerAndCheck) != 1 {
		t.Fatalf("expected results for 1 checker, got %d", len(resultsOrderedByCheckerAndCheck))
	}

	results := resultsOrderedByCheckerAndCheck[0]
	assert.Len(t, results, 1, "expected 1 result from the checker")
	assert.True(t, results[0].InternalError, "expected result to be an error after cancellation")
}

func TestRun_OrderOfResults(t *testing.T) {
	ctx := context.Background()
	cluster := topologyCluster()

	checker1 := &mockChecker{
		checks: []Check{
			&delayedCheck{
				name:  "c1-check1",
				delay: 30 * time.Millisecond,
			},
			&delayedCheck{
				name:  "c1-check2",
				delay: 10 * time.Millisecond,
			},
		},
	}

	checker2 := &mockChecker{
		checks: []Check{
			&delayedCheck{
				name:  "c2-check1",
				delay: 0,
			},
			&delayedCheck{
				name:  "c2-check2",
				delay: 20 * time.Millisecond,
			},
		},
	}

	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), []Checker{checker1, checker2})
	if len(resultsOrderedByCheckerAndCheck) != 2 {
		t.Fatalf("expected results for 2 checkers, got %d", len(resultsOrderedByCheckerAndCheck))
	}

	// The order should be all checks from checker1 followed by all checks from checker2,
	// regardless of their completion time
	expected := []string{"c1-check1", "c1-check2", "c2-check1", "c2-check2"}
	expectedIdx := 0
	for _, results := range resultsOrderedByCheckerAndCheck {
		for _, result := range results {
			assert.Equal(t, expected[expectedIdx], result.Name, "expected result name to match")
			expectedIdx++
		}
	}
}

func TestRun_LargeNumberOfCheckersAndChecks(t *testing.T) {
	ctx := context.Background()
	cluster := topologyCluster()

	checkerCount := 10
	checksPerChecker := 50

	checkers := make([]Checker, checkerCount)
	expectedTotal := checkerCount * checksPerChecker

	for i := 0; i < checkerCount; i++ {
		checks := make([]Check, checksPerChecker)
		for j := 0; j < checksPerChecker; j++ {
			checkerIndex := i
			checkIndex := j
			checks[j] = &mockCheck{
				name: fmt.Sprintf("checker%d-check%d", checkerIndex, checkIndex),
				result: CheckResult{
					Allowed: true,
				},
			}
		}
		checkers[i] = &mockChecker{
			checks: checks,
		}
	}

	start := time.Now()
	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), checkers)
	duration := time.Since(start)

	resultTotal := 0
	for _, results := range resultsOrderedByCheckerAndCheck {
		resultTotal += len(results)
	}

	assert.Equal(t, expectedTotal, resultTotal, "expected total results to match the number of checks")

	t.Logf("Completed %d checks in %v", expectedTotal, duration)
}

func TestRun_ErrorHandlingInChecks(t *testing.T) {
	ctx := context.Background()
	cluster := topologyCluster()

	// Create a checker with a check that returns an error
	errorCheck := &mockCheck{
		name: "error-check",
		result: CheckResult{
			InternalError: true,
			Causes: []Cause{
				{
					Message: "simulated error in check",
					Field:   "spec.errorField",
				},
			},
		},
	}
	checker := &mockChecker{
		checks: []Check{errorCheck},
	}

	// Run the checks
	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), []Checker{checker})
	assert.Len(t, resultsOrderedByCheckerAndCheck, 1, "expected results for 1 checker")
	assert.Len(t, resultsOrderedByCheckerAndCheck[0], 1, "expected 1 result from the checker")

	wantErrorCheckResult := namedResult{
		Name: "error-check",
		CheckResult: CheckResult{
			InternalError: true,
			Causes: []Cause{
				{
					Message: "simulated error in check",
					Field:   "spec.errorField",
				},
			},
		},
	}
	assert.Equal(t, wantErrorCheckResult, resultsOrderedByCheckerAndCheck[0][0], "expected error check result")
}

type panicCheck struct {
	name string
}

func (c *panicCheck) Name() string {
	return c.name
}

func (c *panicCheck) Run(_ context.Context) CheckResult {
	// This should never cause the test to fail due to panic
	panic("simulated panic in check")
}

func TestRun_PanicHandlingInChecks(t *testing.T) {
	ctx := context.Background()

	ctrl.SetLogger(klog.Background())

	cluster := topologyCluster()

	// Create a checker with a panicking check
	normalCheck := &mockCheck{
		name: "normal-check",
		result: CheckResult{
			Allowed: true,
		},
	}
	panicCheck := &panicCheck{name: "panicking-check"}
	checker := &mockChecker{
		checks: []Check{
			normalCheck,
			panicCheck,
		},
	}

	// Run the checks
	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), []Checker{checker})
	assert.Len(t, resultsOrderedByCheckerAndCheck, 1, "expected results for 1 checker")
	assert.Len(t, resultsOrderedByCheckerAndCheck[0], 2, "expected 2 results from the checker")

	wantNormalCheckResult := namedResult{
		Name: "normal-check",
		CheckResult: CheckResult{
			Allowed: true,
		},
	}
	wantPanicCheckResult := namedResult{
		Name: "panicking-check",
		CheckResult: CheckResult{
			InternalError: true,
			Causes: []Cause{
				{
					Message: "The preflight check code had a specific internal error called a \"panic\". This error should not happen under normal circumstances. Please report it, and include the following information: simulated panic in check", ///nolint:lll // Message is long.
					Field:   "",
				},
			},
		},
	}
	assert.Equal(t, wantNormalCheckResult, resultsOrderedByCheckerAndCheck[0][0], "expected normal check result")
	assert.Equal(t, wantPanicCheckResult, resultsOrderedByCheckerAndCheck[0][1], "expected panic check result")
}

func TestRun_ZeroChecksFromChecker(t *testing.T) {
	ctx := context.Background()
	cluster := topologyCluster()

	// Checker that returns no checks
	emptyChecker := &mockChecker{
		checks: []Check{},
	}

	// Checker that returns some checks
	normalChecker := &mockChecker{
		checks: []Check{
			&mockCheck{
				name: "check1",
				result: CheckResult{
					Allowed: true,
				},
			},
		},
	}

	resultsOrderedByCheckerAndCheck := run(ctx, nil, cluster, skip.New(cluster), []Checker{emptyChecker, normalChecker})

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

	assert.Equal(t, "check1", normalResults[0].Name, "expected result name to be 'check1'")
}
