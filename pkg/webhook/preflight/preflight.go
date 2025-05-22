// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

import (
	"context"
	"net/http"
	"sync"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type CheckResult struct {
	Allowed bool
	Error   bool

	Causes   []metav1.StatusCause
	Warnings []string
}

type Check = func(ctx context.Context) CheckResult

type Checker interface {
	// Init decides which of its checks should run for the cluster. It then initializes the checks
	// with common dependencies, such as an infrastructure client. Finally, it returns the initialized checks,
	// ready to be run.
	//
	// Init should not store the context `ctx`, because each check will accept its own context.
	// Checks may use both the client and the cluster.
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

	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Details: &metav1.StatusDetails{},
			},
		},
	}

	// Collect all checks in parallel.
	checkerWG := &sync.WaitGroup{}
	resultCh := make(chan CheckResult)
	for _, checker := range h.checkers {
		checkerWG.Add(1)

		go func(ctx context.Context, checker Checker, resultCh chan CheckResult) {
			// Initialize the checker.
			checks := checker.Init(ctx, h.client, cluster)

			// Run its checks in parallel.
			checksWG := &sync.WaitGroup{}
			for _, check := range checks {
				checksWG.Add(1)
				go func(ctx context.Context, check Check, resultCh chan CheckResult) {
					result := check(ctx)
					resultCh <- result
					checksWG.Done()
				}(ctx, check, resultCh)
			}
			checksWG.Wait()

			checkerWG.Done()
		}(ctx, checker, resultCh)
	}

	// Close the channel when all checkers are done.
	go func(wg *sync.WaitGroup, resultCh chan CheckResult) {
		wg.Wait()
		close(resultCh)
	}(checkerWG, resultCh)

	// Collect all check results from the channel.
	internalError := false
	for result := range resultCh {
		if result.Error {
			internalError = true
		}

		if !result.Allowed {
			resp.Allowed = false
		}

		resp.Result.Details.Causes = append(resp.Result.Details.Causes, result.Causes...)
		resp.Warnings = append(resp.Warnings, result.Warnings...)
	}

	if len(resp.Result.Details.Causes) == 0 {
		return resp
	}

	// Because we have some causes, we construct the response message and code.
	resp.Result.Message = "preflight checks failed"
	resp.Result.Code = http.StatusForbidden
	resp.Result.Reason = metav1.StatusReasonForbidden
	if internalError {
		// Internal errors take precedence over check failures.
		resp.Result.Code = http.StatusInternalServerError
		resp.Result.Reason = metav1.StatusReasonInternalError
	}

	return resp
}
