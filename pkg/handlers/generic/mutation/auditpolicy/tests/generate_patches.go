// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/onsi/gomega"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func TestGeneratePatches(
	t *testing.T, generatorFunc func() mutation.GeneratePatches,
) {
	t.Helper()

	capitest.ValidateGeneratePatches(
		t,
		generatorFunc,
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
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
	)
}
