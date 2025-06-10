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

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/optout"
)

type (
	// CheckerFactory returns a Checker for a given cluster.
	CheckerFactory func(client ctrlclient.Client, cluster *clusterv1.Cluster) Checker
	Checker        interface {
		// Init returns the checks that should run for the cluster.
		Init(ctx context.Context) []Check
	}

	Check       = func(ctx context.Context) CheckResult
	CheckResult struct {
		Name string

		Allowed bool
		Error   bool

		Causes   []Cause
		Warnings []string
	}
	Cause struct {
		Message string
		Field   string
	}
)

type WebhookHandler struct {
	client    ctrlclient.Client
	decoder   admission.Decoder
	factories []CheckerFactory
}

func New(client ctrlclient.Client, decoder admission.Decoder, factories ...CheckerFactory) *WebhookHandler {
	h := &WebhookHandler{
		client:    client,
		decoder:   decoder,
		factories: factories,
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

	if optout.New(cluster).ForAll() {
		// If the cluster has opted out of all checks, return allowed.
		return admission.Allowed("Cluster has opted out of all preflight checks")
	}

	resultsOrderedByCheckerAndCheck := run(ctx, h.client, cluster, h.factories)

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
	factories []CheckerFactory,
) [][]CheckResult {
	resultsOrderedByCheckerAndCheck := make([][]CheckResult, len(factories))

	checkersWG := sync.WaitGroup{}
	for i, factory := range factories {
		checkersWG.Add(1)
		go func(ctx context.Context, client ctrlclient.Client, cluster *clusterv1.Cluster, factory CheckerFactory, i int) {
			checker := factory(client, cluster)

			checks := checker.Init(ctx)
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
		}(
			ctx,
			client,
			cluster,
			factory,
			i,
		)
	}
	checkersWG.Wait()

	return resultsOrderedByCheckerAndCheck
}
