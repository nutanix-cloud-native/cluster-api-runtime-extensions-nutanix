// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/skip"
)

const (
	// Timeout is the duration, in seconds, that the preflight checks handler has to respond.
	// IMPORTANT Keep in sync timeoutSeconds in the kubebuilder:webhook marker defined in this package.
	Timeout = 30 * time.Second
)

type (
	// Checker returns a set of checks that have been initialized with common dependencies,
	// such as an infrastructure API client.
	Checker interface {
		// Init returns the checks that should run for the cluster.
		Init(ctx context.Context, client ctrlclient.Client, cluster *clusterv1.Cluster) []Check
	}

	// Check represents a single preflight check that can be run against a cluster.
	// It has a Name method that returns the name of the check, and a Run method executes
	// the check, and returns a CheckResult.
	// The Name method is used to identify the check if Run fails to return a result, for
	// example if it panics.
	Check interface {
		// Name returns the name of the check.
		// The name should be unique across all checks, and should be used to identify the check
		// in the CheckResult.
		// It is also used to skip the check if the cluster has skipped it.
		Name() string

		// Run executes the check and returns a CheckResult.
		Run(ctx context.Context) CheckResult
	}

	// CheckResult represents the result of a check.
	// It contains the name of the check, a boolean indicating whether the check passed, an
	// error boolean indicating whether there was an internal error running the check, and a
	// list of causes for the failure. It also contains a list of warnings that were
	// generated during the check.
	CheckResult struct {
		// Allowed indicates whether the check passed.
		Allowed bool

		// InternalError indicates whether there was an internal error running the check.
		// This should be false for most check failures. It can be true in case of an unexpected
		// error, like a network error, an API rate-limit error, etc.
		InternalError bool

		// Causes contains a list of causes for the failure. Each cause has a message and an
		// optional field that the cause relates to. The field is used to indicate which part of
		// the cluster configuration the cause relates to.
		Causes []Cause

		// Warnings contains a list of warnings returned by the check.
		// For example, a check should return a warning when the cluster uses configuration
		// not yet supported by the check.
		Warnings []string
	}

	// Cause represents a cause of a check failure. It contains a message and an optional
	// field that the cause relates to. The field is used to indicate which part of the
	// cluster configuration the cause relates to.
	Cause struct {
		// Message is a human-readable message describing the cause of the failure.
		Message string

		// Field is an optional field that the cause relates to.
		// It is used to indicate which part of the cluster configuration the cause relates to.
		// It is a JSONPath expression that points to the field in the cluster configuration.
		// For example, "spec.topology.variables[.name=clusterConfig].value.imageRegistries[0]".
		Field string
	}
)

type WebhookHandler struct {
	client   ctrlclient.Client
	decoder  admission.Decoder
	checkers []Checker
}

func New(client ctrlclient.Client, decoder admission.Decoder, checkers ...Checker) *WebhookHandler {
	h := &WebhookHandler{
		client:   client,
		decoder:  decoder,
		checkers: checkers,
	}
	return h
}

type namedResult struct {
	Name string
	CheckResult
}

func (h *WebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := ctrl.LoggerFrom(ctx)

	if req.Operation == admissionv1.Delete {
		log.V(5).Info("Skipping preflight checks for delete operation")
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	err := h.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Checks run only for ClusterClass-based clusters.
	if cluster.Spec.Topology == nil {
		log.V(5).Info("Skipping preflight checks for non-topology cluster")
		return admission.Allowed("")
	}

	if cluster.Spec.Paused {
		// If the cluster is paused, skip all checks.
		// This allows the cluster to be moved to another API server without running checks.
		log.V(5).Info("Skipping preflight checks for paused cluster")
		return admission.Allowed("")
	}

	skipEvaluator := skip.New(cluster)

	if skipEvaluator.ForAll() {
		log.V(5).Info("Skipping all preflight checks")
		// If the cluster has skipped all checks, return allowed.
		return admission.Allowed("").WithWarnings(
			"Cluster has skipped all preflight checks",
		)
	}

	// Reserve time for checks to handle context cancellation, so
	// that we have time to summarize the results, and return a response.
	checkTimeout := Timeout - 2*time.Second
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, checkTimeout)
	log.V(5).Info("Running preflight checks")
	resultsOrderedByCheckerAndCheck := run(checkCtx, h.client, cluster, skipEvaluator, h.checkers)
	checkCtxCancel()

	// Summarize the results.
	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Details: &metav1.StatusDetails{},
			},
		},
	}
	internalError := false
	for _, results := range resultsOrderedByCheckerAndCheck {
		for _, result := range results {
			if result.InternalError {
				internalError = true
			}
			if !result.Allowed {
				resp.Allowed = false
			}
			for _, cause := range result.Causes {
				resp.Result.Details.Causes = append(resp.Result.Details.Causes,
					metav1.StatusCause{
						Type:    metav1.CauseType(result.Name),
						Message: cause.Message,
						Field:   cause.Field,
					},
				)
			}
			resp.Warnings = append(resp.Warnings, result.Warnings...)
		}
	}

	switch {
	case internalError:
		// Internal errors take precedence over check failures.
		resp.Result.Message = "preflight checks failed due to an internal error"
		resp.Result.Code = http.StatusInternalServerError
		resp.Result.Reason = metav1.StatusReasonInternalError
		log.V(5).Error(nil, "Preflight checks failed due to an internal error", "response", resp)
	case !resp.Allowed:
		// Because the response is not allowed, preflights must have failed.
		resp.Result.Message = "preflight checks failed"
		resp.Result.Code = http.StatusUnprocessableEntity
		resp.Result.Reason = metav1.StatusReasonInvalid
		log.V(5).Info("Preflight checks failed", "response", resp)
	default:
		log.V(5).Info("Preflight checks passed", "response", resp)
	}

	return resp
}

// run runs all checks for the cluster, concurrently, and returns the results ordered by checker and check.
// Checker are initialized concurrently, and checks runs concurrently as well.
func run(ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	skipEvaluator *skip.Evaluator,
	checkers []Checker,
) [][]namedResult {
	resultsOrderedByCheckerAndCheck := make([][]namedResult, len(checkers))

	checkersWG := sync.WaitGroup{}
	for i, checker := range checkers {
		checkersWG.Add(1)
		go func(
			ctx context.Context,
			client ctrlclient.Client,
			cluster *clusterv1.Cluster,
			skipEvaluator *skip.Evaluator,
			checker Checker,
			i int,
		) {
			defer checkersWG.Done()

			checks := checker.Init(ctx, client, cluster)
			resultsOrderedByCheck := make([]namedResult, len(checks))

			checksWG := sync.WaitGroup{}
			for j, check := range checks {
				ctrl.LoggerFrom(ctx).V(5).Info(
					"running preflight check",
					"checkName", check.Name(),
				)
				if skipEvaluator.For(check.Name()) {
					ctrl.LoggerFrom(ctx).V(5).Info(
						"Skipping preflight check",
						"checkName", check.Name(),
					)
					resultsOrderedByCheck[j] = namedResult{
						Name: check.Name(),
						CheckResult: CheckResult{
							Allowed:       true,
							InternalError: false,
							Causes:        nil,
							Warnings: []string{
								fmt.Sprintf("Cluster has skipped preflight check %q", check.Name()),
							},
						},
					}
					continue
				}
				checksWG.Add(1)
				go func(
					ctx context.Context,
					check Check,
					j int,
				) {
					defer checksWG.Done()
					defer func() {
						if r := recover(); r != nil {
							resultsOrderedByCheck[j] = namedResult{
								Name: check.Name(),
								CheckResult: CheckResult{
									InternalError: true,
									Causes: []Cause{
										{
											Message: fmt.Sprintf(
												"The preflight check code had a specific internal error called a \"panic\". This error should not happen under normal circumstances. Please report it, and include the following information: %s", ///nolint:lll // Message is long.
												r,
											),
											Field: "",
										},
									},
								},
							}
							ctrl.LoggerFrom(ctx).Error(
								fmt.Errorf("preflight check panic"),
								fmt.Sprintf("%v", r),
								"checkName", check.Name(),
								"stackTrace", string(debug.Stack()),
							)
						}
					}()
					result := check.Run(ctx)
					resultsOrderedByCheck[j] = namedResult{
						Name:        check.Name(),
						CheckResult: result,
					}
				}(ctx, check, j)
			}
			checksWG.Wait()
			resultsOrderedByCheckerAndCheck[i] = resultsOrderedByCheck
		}(
			ctx,
			client,
			cluster,
			skipEvaluator,
			checker,
			i,
		)
	}
	checkersWG.Wait()

	return resultsOrderedByCheckerAndCheck
}
