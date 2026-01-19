// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"os"
	"testing"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
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
				// Start the environment with this to ignore in order to create the pre-existing objects.
				FailurePolicy: ptr.To(admissionv1.Ignore),
			},
		},
	}
	validatingWebhook := &admissionv1.ValidatingWebhookConfiguration{
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
				// Start the environment with this to ignore in order to create the pre-existing objects.
				FailurePolicy: ptr.To(admissionv1.Ignore),
			},
		},
	}

	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M: m,
		EnvironmentOpts: []envtest.EnvironmentOpt{
			envtest.WithPreexistingObjects(
				// Create a ClusterClass for topology-enabled clusters to reference.
				&clusterv1.ClusterClass{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy-class",
						Namespace: metav1.NamespaceDefault,
					},
					Spec: clusterv1.ClusterClassSpec{},
				},
				// Create a pre-existing object without topology or the UUID annotation.
				&clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "preexisting-without-topology-or-uuid-annotation",
						Namespace: metav1.NamespaceDefault,
					},
				},
				// Create a pre-existing object with topology but without the UUID annotation.
				&clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "preexisting-with-topology-without-uuid-annotation",
						Namespace: metav1.NamespaceDefault,
					},
					Spec: clusterv1.ClusterSpec{
						Topology: &clusterv1.Topology{
							Class:   "dummy-class",
							Version: "v1.28.0",
						},
					},
				},
			),
		},
		WebhookInstallOptions: ctrlenvtest.WebhookInstallOptions{
			MutatingWebhooks: []*admissionv1.MutatingWebhookConfiguration{
				mutatingWebhook,
			},
			ValidatingWebhooks: []*admissionv1.ValidatingWebhookConfiguration{
				validatingWebhook,
			},
		},
		SetupEnv: func(e *envtest.Environment) {
			e.GetWebhookServer().Register("/mutate-v1beta1-cluster", &webhook.Admission{
				Handler: NewDefaulter(e.GetClient(), admission.NewDecoder(e.GetScheme())),
			})
			e.GetWebhookServer().Register("/validate-v1beta1-cluster", &webhook.Admission{
				Handler: NewValidator(e.GetClient(), admission.NewDecoder(e.GetScheme())),
			})

			// The webhooks are initially installed with Ignore failure policy above to allow creating objects before
			// actually registering the webhooks so now the webhooks are installed above we can the webhooks to failure
			// policy to ensure they fail if not installed or called correctly.
			mutatingWebhook.Webhooks[0].FailurePolicy = ptr.To(admissionv1.Fail)
			err := e.GetClient().Update(context.Background(), mutatingWebhook)
			if err != nil {
				klog.Fatalf("failed to update the mutating webhook failure policy: %v", err)
			}
			validatingWebhook.Webhooks[0].FailurePolicy = ptr.To(admissionv1.Fail)
			err = e.GetClient().Update(context.Background(), validatingWebhook)
			if err != nil {
				klog.Fatalf("failed to update the validating webhook failure policy: %v", err)
			}

			env = e
		},
	}))
}
