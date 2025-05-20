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
	Field   string
	Message string
	Warning string
	Error   bool
}

type Check = func(ctx context.Context) CheckResult

type Checker interface {
	Checks(ctx context.Context, client ctrlclient.Client, cluster *clusterv1.Cluster) ([]Check, error)
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

	// Initialize checkers in parallel.
	type ChecksResult struct {
		checks []Check
		err    error
	}
	checksResultCh := make(chan ChecksResult, len(h.checkers))

	wg := &sync.WaitGroup{}
	for _, checker := range h.checkers {
		wg.Add(1)
		result := ChecksResult{}
		result.checks, result.err = checker.Checks(ctx, h.client, cluster)
		checksResultCh <- result
		wg.Done()
	}
	wg.Wait()
	close(checksResultCh)

	// Collect all checks.
	checks := make([]Check, 0)
	for checksResult := range checksResultCh {
		if checksResult.err != nil {
			resp.Allowed = false
			resp.Result.Code = http.StatusInternalServerError
			resp.Result.Message = "failed to initialize preflight checks"
			resp.Result.Details.Causes = append(resp.Result.Details.Causes, metav1.StatusCause{
				Type:    metav1.CauseTypeInternal,
				Message: checksResult.err.Error(),
				Field:   "", // This concerns the whole cluster.
			})
			continue
		}
		checks = append(checks, checksResult.checks...)
	}

	// Run all checks in parallel.
	resultCh := make(chan CheckResult, len(checks))
	for _, check := range checks {
		wg.Add(1)
		go func(ctx context.Context, check Check) {
			result := check(ctx)
			resultCh <- result
			wg.Done()
		}(ctx, check)
	}
	wg.Wait()
	close(resultCh)

	// Collect check results.
	for result := range resultCh {
		if result.Error {
			resp.Allowed = false
			resp.Result.Code = http.StatusForbidden
			resp.Result.Message = "preflight checks failed"
			resp.Result.Details.Causes = append(resp.Result.Details.Causes, metav1.StatusCause{
				Type:    metav1.CauseTypeInternal,
				Field:   result.Field,
				Message: result.Message,
			})
			continue
		}

		if !result.Allowed {
			resp.Allowed = false
			resp.Result.Code = http.StatusForbidden
			resp.Result.Message = "preflight checks failed"
			resp.Result.Details.Causes = append(resp.Result.Details.Causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Field:   result.Field,
				Message: result.Message,
			})
		}

		if result.Warning != "" {
			resp.Warnings = append(resp.Warnings, result.Warning)
		}
	}

	return resp
}
