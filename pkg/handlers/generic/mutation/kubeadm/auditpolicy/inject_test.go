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
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
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
							gomega.ContainElements(
								gomega.HaveKeyWithValue("name", "audit-log-maxbackup"),
								gomega.HaveKeyWithValue("name", "audit-log-maxsize"),
								gomega.HaveKeyWithValue("name", "audit-log-path"),
								gomega.HaveKeyWithValue("name", "audit-policy-file"),
								gomega.HaveKeyWithValue("name", "audit-log-maxage"),
								gomega.HaveKeyWithValue("name", "audit-log-compress"),
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
