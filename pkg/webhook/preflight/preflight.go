// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
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
	Provider() string
}

type WebhookHandler struct {
	client             ctrlclient.Client
	decoder            admission.Decoder
	checkersByProvider map[string]Checker
}

func New(client ctrlclient.Client, decoder admission.Decoder, checkers ...Checker) *WebhookHandler {
	h := &WebhookHandler{
		client:             client,
		decoder:            decoder,
		checkersByProvider: make(map[string]Checker, len(checkers)),
	}
	for _, checker := range checkers {
		h.checkersByProvider[checker.Provider()] = checker
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

	// Checks run only for the known infrastructure providers.
	checker, ok := h.checkersByProvider[utils.GetProvider(cluster)]
	if !ok {
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

	checks, err := checker.Checks(ctx, h.client, cluster)
	if err != nil {
		resp.Allowed = false
		resp.Result.Code = http.StatusInternalServerError
		resp.Result.Message = "failed to initialize preflight checks"
		resp.Result.Details.Causes = append(resp.Result.Details.Causes, metav1.StatusCause{
			Type:    metav1.CauseTypeInternal,
			Message: err.Error(),
			Field:   "", // This concerns the whole cluster.
		})
		return resp
	}

	if len(checks) == 0 {
		return admission.Allowed("")
	}

	// Run all checks and collect results.
	// TODO Parallelize	checks.
	for _, check := range checks {
		result := check(ctx)

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
