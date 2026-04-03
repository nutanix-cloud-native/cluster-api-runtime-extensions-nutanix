// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestAuditPolicyPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Audit Policy mutator suite")
}

var _ = Describe("Generate Audit Policy patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name:        "auditpolicy set for KubeadmControlPlaneTemplate",
			RequestItem: request.NewKubeadmControlPlaneTemplateV1Beta1RequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElements(
					gomega.HaveKeyWithValue(
						"path", "/etc/kubernetes/audit-policy.yaml",
					),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
				ValueMatcher: gomega.HaveKeyWithValue(
					"apiServer",
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.SatisfyAll(
								gomega.HaveKey("audit-log-maxbackup"),
								gomega.HaveKey("audit-log-maxsize"),
								gomega.HaveKey("audit-log-path"),
								gomega.HaveKey("audit-policy-file"),
								gomega.HaveKey("audit-log-maxage"),
								gomega.HaveKey("audit-log-compress"),
							),
						),
						gomega.HaveKeyWithValue(
							"extraVolumes",
							[]any{
								map[string]any{
									"hostPath":  "/etc/kubernetes/audit-policy.yaml",
									"mountPath": "/etc/kubernetes/audit-policy.yaml",
									"name":      "audit-policy",
									"readOnly":  true,
									"pathType":  "File",
								},
								map[string]any{
									"name":      "audit-logs",
									"hostPath":  "/var/log/kubernetes/audit",
									"mountPath": "/var/log/audit/",
									"pathType":  "DirectoryOrCreate",
								},
							},
						),
					),
				),
			}},
		},
	}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
