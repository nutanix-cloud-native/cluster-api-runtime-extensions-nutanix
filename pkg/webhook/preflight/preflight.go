// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Cause struct {
	Message string
	Field   string
}

type CheckResult struct {
	Name string

	Allowed bool
	Error   bool

	Causes   []Cause
	Warnings []string
}

type Check = func(ctx context.Context) CheckResult

type Checker interface {
	// Init decides which of its checks should run for the cluster. It then initializes the checks
	// with common dependencies, such as an infrastructure client. Finally, it returns the initialized checks,
	// ready to be run.
	//
	// Init can store the client and cluster, but not the context, because each check will accept its own context.
	Init(ctx context.Context, client ctrlclient.Client, cluster *clusterv1.Cluster) []Check
}

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

func (h *WebhookHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation == admissionv1.Delete {
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

	resultsOrderedByCheckerAndCheck := run(ctx, h.client, cluster, h.checkers)

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
			if result.Error {
				internalError = true
			}
			if !result.Allowed {
				resp.Allowed = false
			}
			for _, cause := range result.Causes {
				resp.Result.Details.Causes = append(resp.Result.Details.Causes, metav1.StatusCause{
					Type:    metav1.CauseType(fmt.Sprintf("FailedPreflight%s", result.Name)),
					Message: cause.Message,
					Field:   cause.Field,
				})
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
	checkers []Checker,
) [][]CheckResult {
	resultsOrderedByCheckerAndCheck := make([][]CheckResult, len(checkers))

	checkersWG := sync.WaitGroup{}
	for i, checker := range checkers {
		checkersWG.Add(1)
		go func(ctx context.Context, client ctrlclient.Client, cluster *clusterv1.Cluster, checker Checker, i int) {
			checks := checker.Init(ctx, client, cluster)
			resultsOrderedByCheck := make([]CheckResult, len(checks))

			checksWG := sync.WaitGroup{}
			for j, check := range checks {
				checksWG.Add(1)
				go func(ctx context.Context, check Check, j int) {
					result := check(ctx)
					resultsOrderedByCheck[j] = result
					checksWG.Done()
				}(ctx, check, j)
			}
			checksWG.Wait()
			resultsOrderedByCheckerAndCheck[i] = resultsOrderedByCheck

			checkersWG.Done()
		}(ctx, client, cluster, checker, i)
	}
	checkersWG.Wait()

	return resultsOrderedByCheckerAndCheck
}
