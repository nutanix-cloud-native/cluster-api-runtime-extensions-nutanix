// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func TestAuditPolicyPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Audit Policy mutator suite")
}

var _ = Describe("Generate Audit Policy patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name:        "auditpolicy set for KubeadmControlPlaneTemplate",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElements(
					gomega.HaveKeyWithValue(
						"path", "/etc/kubernetes/audit-policy/apiserver-audit-policy.yaml",
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
							map[string]interface{}{
								"audit-log-maxbackup": "10",
								"audit-log-maxsize":   "100",
								"audit-log-path":      "/var/log/audit/kube-apiserver-audit.log",
								"audit-policy-file":   "/etc/kubernetes/audit-policy/apiserver-audit-policy.yaml",
								"audit-log-maxage":    "30",
							},
						),
						gomega.HaveKeyWithValue(
							"extraVolumes",
							[]interface{}{
								map[string]interface{}{
									"hostPath":  "/etc/kubernetes/audit-policy/",
									"mountPath": "/etc/kubernetes/audit-policy/",
									"name":      "audit-policy",
									"readOnly":  true,
								},
								map[string]interface{}{
									"name":      "audit-logs",
									"hostPath":  "/var/log/kubernetes/audit",
									"mountPath": "/var/log/audit/",
								},
							},
						),
					),
				),
			}},
		},
	}

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
