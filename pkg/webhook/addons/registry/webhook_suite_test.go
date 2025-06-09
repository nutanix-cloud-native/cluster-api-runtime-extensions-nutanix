// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"os"
	"testing"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlenvtest "sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/envtest"
)

var (
	ctx = ctrl.SetupSignalHandler()
	env *envtest.Environment
)

func TestMain(m *testing.M) {
	mutatingWebhook := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "registry-defaulter.caren.nutanix.com",
		},
		Webhooks: []admissionv1.MutatingWebhook{
			{
				Name: "registry-defaulter.caren.nutanix.com",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Path: ptr.To("/mutate-v1beta1-registry-addon"),
					},
				},
				Rules: []admissionv1.RuleWithOperations{{
					Operations: []admissionv1.OperationType{
						admissionv1.Create,
					},
					Rule: admissionv1.Rule{
						APIGroups:   []string{"cluster.x-k8s.io"},
						APIVersions: []string{"*"},
						Resources:   []string{"clusters"},
					},
				}},
				SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
				AdmissionReviewVersions: []string{"v1"},
				FailurePolicy:           ptr.To(admissionv1.Fail),
			},
		},
	}

	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M: m,
		WebhookInstallOptions: ctrlenvtest.WebhookInstallOptions{
			MutatingWebhooks: []*admissionv1.MutatingWebhookConfiguration{
				mutatingWebhook,
			},
		},
		SetupEnv: func(e *envtest.Environment) {
			e.GetWebhookServer().Register("/mutate-v1beta1-registry-addon", &webhook.Admission{
				Handler: NewWorkloadClusterAutoEnabler(e.GetClient(), admission.NewDecoder(e.GetScheme())).Defaulter(),
			})

			env = e
		},
	}))
}
