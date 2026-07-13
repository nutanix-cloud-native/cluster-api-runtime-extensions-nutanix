// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditlog

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

const testPolicyYAML = `apiVersion: audit.k8s.io/v1
kind: Policy
rules: []
`

const testWebhookSecretName = "test-audit-webhook-kubeconfig"

// testWebhookInitialBackoff is a positive InitialBackoff wired into the webhook backend Ginkgo case.
var testWebhookInitialBackoff = metav1.Duration{Duration: 10 * time.Second}

// testEventBatching uses distinct values so batch-related apiserver flags can be asserted in tests.
var testEventBatching = &v1alpha1.AuditLogEventBatching{
	BufferSize:     1000,
	MaxSize:        50,
	MaxWait:        30,
	ThrottleEnable: true,
	ThrottleQPS:    5,
	ThrottleBurst:  10,
}

func TestAuditLogPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	// Ginkgo allows only one RunSpecs per package; file-backend and webhook-backend are separate Describe blocks below.
	RunSpecs(t, "Audit log mutator suite")
}

var _ = Describe("Generate Audit log patches (file backend)", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1beta2.AddToScheme(clientScheme))

	patchGenerator := func() mutation.GeneratePatches {
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			client,
			NewPatch(client),
		).(mutation.GeneratePatches)
	}

	auditLogVars := []runtimehooksv1.Variable{
		capitest.VariableWithValue(
			v1alpha1.ClusterConfigVariableName,
			v1alpha1.AuditLog{
				Log: &v1alpha1.AuditLogBackendLog{
					Mode:          "batch",
					MaxAge:        30,
					MaxBackup:     90,
					MaxSize:       100,
					Compress:      true,
					EventBatching: testEventBatching,
				},
				Policy: &v1alpha1.AuditLogPolicy{
					ConfigMap: &v1alpha1.LocalObjectReference{Name: "test-audit-policy"},
				},
			},
			VariableName,
		),
	}

	logBackendMatchers := []capitest.JSONPatchMatcher{
		{
			Operation: "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/files",
			ValueMatcher: gomega.ContainElements(
				gomega.HaveKeyWithValue("path", auditPolicyPath),
			),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
			ValueMatcher: gomega.HaveKeyWithValue(
				"apiServer",
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue(
						"extraArgs",
						gomega.ContainElements(
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-maxbackup"),
								gomega.HaveKeyWithValue("value", "90"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-maxsize"),
								gomega.HaveKeyWithValue("value", "100"),
							),
							gomega.HaveKeyWithValue("name", "audit-log-path"),
							gomega.HaveKeyWithValue("name", "audit-policy-file"),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-maxage"),
								gomega.HaveKeyWithValue("value", "30"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-compress"),
								gomega.HaveKeyWithValue("value", "true"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-mode"),
								gomega.HaveKeyWithValue("value", "batch"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-batch-buffer-size"),
								gomega.HaveKeyWithValue("value", "1000"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-batch-max-size"),
								gomega.HaveKeyWithValue("value", "50"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-batch-max-wait"),
								gomega.HaveKeyWithValue("value", "30s"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-batch-throttle-enable"),
								gomega.HaveKeyWithValue("value", "true"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-batch-throttle-qps"),
								gomega.HaveKeyWithValue("value", "5"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-log-batch-throttle-burst"),
								gomega.HaveKeyWithValue("value", "10"),
							),
						),
					),
					gomega.HaveKeyWithValue(
						"extraVolumes",
						gomega.ContainElements(
							gomega.HaveKeyWithValue("name", "audit-policy"),
							gomega.HaveKeyWithValue("name", "audit-logs"),
						),
					),
				),
			),
		},
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
	}

	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}

	Context("with audit policy ConfigMap", func() {
		BeforeEach(func(ctx SpecContext) {
			client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(client.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-audit-policy",
					Namespace: request.Namespace,
				},
				Data: map[string]string{
					AuditPolicyDataKey: testPolicyYAML,
				},
			})).To(gomega.Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(client.Delete(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-audit-policy",
					Namespace: request.Namespace,
				},
			})).To(gomega.Succeed())
		})

		It("audit log file backend with batching for KubeadmControlPlaneTemplate", func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &capitest.PatchTestDef{
				Vars:                  auditLogVars,
				RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: logBackendMatchers,
			})
		})
	})
})

var _ = Describe("Generate Audit log patches (webhook backend)", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1beta2.AddToScheme(clientScheme))

	patchGenerator := func() mutation.GeneratePatches {
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			client,
			NewPatch(client),
		).(mutation.GeneratePatches)
	}

	auditWebhookVars := []runtimehooksv1.Variable{
		capitest.VariableWithValue(
			v1alpha1.ClusterConfigVariableName,
			v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:          "batch",
					Secret:        &v1alpha1.LocalObjectReference{Name: testWebhookSecretName},
					EventBatching: testEventBatching,
				},
				Policy: &v1alpha1.AuditLogPolicy{
					ConfigMap: &v1alpha1.LocalObjectReference{Name: "test-audit-policy"},
				},
			},
			VariableName,
		),
	}

	auditWebhookVarsWithInitialBackoff := []runtimehooksv1.Variable{
		capitest.VariableWithValue(
			v1alpha1.ClusterConfigVariableName,
			v1alpha1.AuditLog{
				Webhook: &v1alpha1.AuditLogBackendWebhook{
					Mode:           "batch",
					Secret:         &v1alpha1.LocalObjectReference{Name: testWebhookSecretName},
					InitialBackoff: testWebhookInitialBackoff,
					EventBatching:  testEventBatching,
				},
				Policy: &v1alpha1.AuditLogPolicy{
					ConfigMap: &v1alpha1.LocalObjectReference{Name: "test-audit-policy"},
				},
			},
			VariableName,
		),
	}

	webhookBackendMatchers := []capitest.JSONPatchMatcher{
		{
			Operation: "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/files",
			ValueMatcher: gomega.ContainElements(
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue("path", auditPolicyPath),
				),
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue("path", webhookKubeconfigPath),
					gomega.HaveKeyWithValue(
						"contentFrom",
						gomega.HaveKeyWithValue(
							"secret",
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", testWebhookSecretName),
								gomega.HaveKeyWithValue("key", WebhookKubeconfigSecretKey),
							),
						),
					),
				),
			),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
			ValueMatcher: gomega.HaveKeyWithValue(
				"apiServer",
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue(
						"extraArgs",
						gomega.ContainElements(
							gomega.HaveKeyWithValue("name", "audit-policy-file"),
							gomega.HaveKeyWithValue("name", "audit-webhook-config-file"),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-mode"),
								gomega.HaveKeyWithValue("value", "batch"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-buffer-size"),
								gomega.HaveKeyWithValue("value", "1000"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-max-size"),
								gomega.HaveKeyWithValue("value", "50"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-max-wait"),
								gomega.HaveKeyWithValue("value", "30s"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-throttle-enable"),
								gomega.HaveKeyWithValue("value", "true"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-throttle-qps"),
								gomega.HaveKeyWithValue("value", "5"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-throttle-burst"),
								gomega.HaveKeyWithValue("value", "10"),
							),
						),
					),
					gomega.HaveKeyWithValue(
						"extraVolumes",
						gomega.ContainElements(
							gomega.HaveKeyWithValue("name", "audit-policy"),
							gomega.HaveKeyWithValue("name", "audit-webhook-kubeconfig"),
						),
					),
				),
			),
		},
	}

	webhookBackendMatchersWithInitialBackoff := []capitest.JSONPatchMatcher{
		webhookBackendMatchers[0],
		{
			Operation: "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
			ValueMatcher: gomega.HaveKeyWithValue(
				"apiServer",
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue(
						"extraArgs",
						gomega.ContainElements(
							gomega.HaveKeyWithValue("name", "audit-policy-file"),
							gomega.HaveKeyWithValue("name", "audit-webhook-config-file"),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-mode"),
								gomega.HaveKeyWithValue("value", "batch"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-initial-backoff"),
								gomega.HaveKeyWithValue("value", testWebhookInitialBackoff.Duration.String()),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-buffer-size"),
								gomega.HaveKeyWithValue("value", "1000"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-max-size"),
								gomega.HaveKeyWithValue("value", "50"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-max-wait"),
								gomega.HaveKeyWithValue("value", "30s"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-throttle-enable"),
								gomega.HaveKeyWithValue("value", "true"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-throttle-qps"),
								gomega.HaveKeyWithValue("value", "5"),
							),
							gomega.SatisfyAll(
								gomega.HaveKeyWithValue("name", "audit-webhook-batch-throttle-burst"),
								gomega.HaveKeyWithValue("value", "10"),
							),
						),
					),
					gomega.HaveKeyWithValue(
						"extraVolumes",
						gomega.ContainElements(
							gomega.HaveKeyWithValue("name", "audit-policy"),
							gomega.HaveKeyWithValue("name", "audit-webhook-kubeconfig"),
						),
					),
				),
			),
		},
	}

	Context("with audit policy ConfigMap", func() {
		BeforeEach(func(ctx SpecContext) {
			client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(client.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-audit-policy",
					Namespace: request.Namespace,
				},
				Data: map[string]string{
					AuditPolicyDataKey: testPolicyYAML,
				},
			})).To(gomega.Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(client.Delete(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-audit-policy",
					Namespace: request.Namespace,
				},
			})).To(gomega.Succeed())
		})

		It("audit webhook backend with batching for KubeadmControlPlaneTemplate", func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &capitest.PatchTestDef{
				Vars:                  auditWebhookVars,
				RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: webhookBackendMatchers,
			})
		})

		It("audit webhook backend with initial backoff and batching for KubeadmControlPlaneTemplate", func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &capitest.PatchTestDef{
				Vars:                  auditWebhookVarsWithInitialBackoff,
				RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: webhookBackendMatchersWithInitialBackoff,
			})
		})
	})
})
