package multi

import (
	"context"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type mergingMultiValidating []admission.Handler

func (hs mergingMultiValidating) Handle(ctx context.Context, req admission.Request) admission.Response {
	allowed := true
	errored := false
	messages := []string{}
	causes := []metav1.StatusCause{}
	for _, handler := range hs {
		resp := handler.Handle(ctx, req)
		if !resp.Allowed {
			allowed = false

			if resp.Result.Code != http.StatusForbidden {
				errored = true
			}

			if len(resp.Result.Message) > 0 {
				messages = append(messages, resp.Result.Message)
			}

			if resp.Result.Details != nil {
				causes = append(causes, resp.Result.Details.Causes...)
			}
		}
	}

	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: allowed,
			Result: &metav1.Status{
				Code:   http.StatusOK,
				Reason: "",
			},
		},
	}
	if !allowed {
		resp.AdmissionResponse.Result.Details = &metav1.StatusDetails{
			Name:   req.Name,
			Group:  req.Kind.Group,
			Kind:   req.Kind.Kind,
			Causes: causes,
		}
		resp.AdmissionResponse.Result.Code = http.StatusForbidden
		resp.Result.Reason = metav1.StatusReasonForbidden
	}
	if errored {
		resp.AdmissionResponse.Result.Code = http.StatusUnprocessableEntity
	}
	resp.Result.Message = strings.Join(messages, ", ")

	return resp
}

// MultiValidatingHandler combines multiple validating webhook handlers into a single
// validating webhook handler.  Handlers are called in sequential order, and the first
// `allowed: false`	response may short-circuit the rest.
func MultiValidatingHandler(handlers ...admission.Handler) admission.Handler {
	return mergingMultiValidating(handlers)
}
