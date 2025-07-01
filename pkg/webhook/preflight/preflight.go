// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/skip"
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
		Name() string
		Run(ctx context.Context) CheckResult
	}

	// CheckResult represents the result of a check.
	// It contains the name of the check, a boolean indicating whether the check passed, an
	// error boolean indicating whether there was an internal error running the check, and a
	// list of causes for the failure. It also contains a list of warnings that were
	// generated during the check.
	CheckResult struct {
		Allowed       bool
		InternalError bool

		Causes   []Cause
		Warnings []string
	}

	// Cause represents a cause of a check failure. It contains a message and an optional
	// field that the cause relates to. The field is used to indicate which part of the
	// cluster configuration the cause relates to.
	Cause struct {
		Message string
		Field   string
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
	if req.Operation == admissionv1.Delete {
		return admission.Allowed("")
	}

	if req.Operation == admissionv1.Update {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	err := h.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Checks run only for ClusterClass-based clusters.
	if cluster.Spec.Topology == nil {
		return admission.Allowed("")
	}

	if cluster.Spec.Paused {
		// If the cluster is paused, skip all checks.
		// This allows the cluster to be moved to another API server without running checks.
		return admission.Allowed("")
	}

	skipEvaluator := skip.New(cluster)

	if skipEvaluator.ForAll() {
		// If the cluster has skipped all checks, return allowed.
		return admission.Allowed("").WithWarnings(
			"Cluster has skipped all preflight checks",
		)
	}

	resultsOrderedByCheckerAndCheck := run(ctx, h.client, cluster, skipEvaluator, h.checkers)

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
	case !resp.Allowed:
		// Because the response is not allowed, preflights must have failed.
		resp.Result.Message = "preflight checks failed"
		resp.Result.Code = http.StatusUnprocessableEntity
		resp.Result.Reason = metav1.StatusReasonInvalid
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
					"clusterName", cluster.Name,
					"clusterNamespace", cluster.Namespace,
				)
				if skipEvaluator.For(check.Name()) {
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
											Message: fmt.Sprintf("internal error (panic): %s", r),
											Field:   "",
										},
									},
								},
							}
							ctrl.LoggerFrom(ctx).Error(
								fmt.Errorf("preflight check panic"),
								fmt.Sprintf("%v", r),
								"checkName", check.Name(),
								"clusterName", cluster.Name,
								"clusterNamespace", cluster.Namespace,
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
