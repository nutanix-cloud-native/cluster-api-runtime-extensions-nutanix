// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

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
	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M: m,
		SetupEnv: func(e *envtest.Environment) {
			e.GetWebhookServer().Register("/mutate-v1beta1-cluster", &webhook.Admission{
				Handler: NewDefaulter(e.GetClient(), admission.NewDecoder(e.GetScheme())),
			})
			e.GetWebhookServer().Register("/validate-v1beta1-cluster", &webhook.Admission{
				Handler: NewValidator(e.GetClient(), admission.NewDecoder(e.GetScheme())),
			})
			env = e
		},
		WebhookInstallOptions: ctrlenvtest.WebhookInstallOptions{
			MutatingWebhooks: []*admissionv1.MutatingWebhookConfiguration{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-defaulter.caren.nutanix.com",
				},
				Webhooks: []admissionv1.MutatingWebhook{
					{
						Name: "cluster-defaulter.caren.nutanix.com",
						ClientConfig: admissionv1.WebhookClientConfig{
							Service: &admissionv1.ServiceReference{
								Path: ptr.To("/mutate-v1beta1-cluster"),
							},
						},
						Rules: []admissionv1.RuleWithOperations{{
							Operations: []admissionv1.OperationType{
								admissionv1.Create,
								admissionv1.Update,
							},
							Rule: admissionv1.Rule{
								APIGroups:   []string{"cluster.x-k8s.io"},
								APIVersions: []string{"*"},
								Resources:   []string{"clusters"},
							},
						}},
						SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
						AdmissionReviewVersions: []string{"v1"},
					},
				},
			}},
			ValidatingWebhooks: []*admissionv1.ValidatingWebhookConfiguration{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-validator.caren.nutanix.com",
				},
				Webhooks: []admissionv1.ValidatingWebhook{
					{
						Name: "cluster-validator.caren.nutanix.com",
						ClientConfig: admissionv1.WebhookClientConfig{
							Service: &admissionv1.ServiceReference{
								Path: ptr.To("/validate-v1beta1-cluster"),
							},
						},
						Rules: []admissionv1.RuleWithOperations{{
							Operations: []admissionv1.OperationType{
								admissionv1.Create,
								admissionv1.Update,
							},
							Rule: admissionv1.Rule{
								APIGroups:   []string{"cluster.x-k8s.io"},
								APIVersions: []string{"*"},
								Resources:   []string{"clusters"},
							},
						}},
						SideEffects:             ptr.To(admissionv1.SideEffectClassNone),
						AdmissionReviewVersions: []string{"v1"},
					},
				},
			}},
		},

		// 			- admissionReviewVersions:
		//   - v1
		// clientConfig:
		//   service:
		//     name: '{{ include "chart.name" . }}-admission'
		//     namespace: '{{ .Release.Namespace }}'
		//     path: /mutate-v1beta1-cluster
		// failurePolicy: Fail
		// name: cluster-defaulter.caren.nutanix.com
		// rules:
		//   - apiGroups:
		//       - cluster.x-k8s.io
		//     apiVersions:
		//       - '*'
		//     operations:
		//       - CREATE
		//       - UPDATE
		//     resources:
		//       - clusters
		// sideEffects: None
	}))
}
