// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package podsecurityadmission

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/admissionconfiguration"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestPodSecurityAdmissionPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Pod Security Admission mutator suite")
}

var _ = Describe("Generate Pod Security Admission patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler(
			"", helpers.TestEnv.Client, NewPatch(),
		).(mutation.GeneratePatches)
	}

	expectedAdmissionConfigContent := `apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: PodSecurity
  path: /etc/kubernetes/pod-security-admission.yaml
`

	admissionConfigFileMatcher := gomega.SatisfyAll(
		gomega.HaveKeyWithValue("path", admissionconfiguration.DefaultAdmissionConfigPath),
		gomega.HaveKeyWithValue("content", expectedAdmissionConfigContent),
	)

	testDefs := []capitest.PatchTestDef{
		{
			Name:                  "variable not set results in no patches",
			RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
		},
		{
			Name:        "enforce restricted with defaults",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						admissionConfigFileMatcher,
						gomega.SatisfyAll(
							gomega.HaveKeyWithValue("path", psaConfigFilePath),
							gomega.HaveKeyWithValue("content",
								`apiVersion: pod-security.admission.config.k8s.io/v1
kind: PodSecurityConfiguration
defaults:
  enforce: "restricted"
  enforce-version: "latest"
  audit: "privileged"
  audit-version: "latest"
  warn: "privileged"
  warn-version: "latest"
exemptions:
  namespaces:
    - "kube-system"
`,
							),
						),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.PodSecurityAdmission{
						Enforce: v1alpha1.PodSecurityStandardRestricted,
						Audit:   v1alpha1.PodSecurityStandardPrivileged,
						Warn:    v1alpha1.PodSecurityStandardPrivileged,
						Exemptions: v1alpha1.PodSecurityExemptions{
							Namespaces: []string{"kube-system"},
						},
					},
					VariableName,
				),
			},
		},
		{
			Name:        "all modes set to restricted",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						admissionConfigFileMatcher,
						gomega.SatisfyAll(
							gomega.HaveKeyWithValue("path", psaConfigFilePath),
							gomega.HaveKeyWithValue("content",
								`apiVersion: pod-security.admission.config.k8s.io/v1
kind: PodSecurityConfiguration
defaults:
  enforce: "restricted"
  enforce-version: "latest"
  audit: "restricted"
  audit-version: "latest"
  warn: "restricted"
  warn-version: "latest"
exemptions:
  namespaces:
    - "kube-system"
`,
							),
						),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.PodSecurityAdmission{
						Enforce: v1alpha1.PodSecurityStandardRestricted,
						Audit:   v1alpha1.PodSecurityStandardRestricted,
						Warn:    v1alpha1.PodSecurityStandardRestricted,
						Exemptions: v1alpha1.PodSecurityExemptions{
							Namespaces: []string{"kube-system"},
						},
					},
					VariableName,
				),
			},
		},
		{
			Name:        "custom exemptions",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						admissionConfigFileMatcher,
						gomega.SatisfyAll(
							gomega.HaveKeyWithValue("path", psaConfigFilePath),
							gomega.HaveKeyWithValue("content",
								`apiVersion: pod-security.admission.config.k8s.io/v1
kind: PodSecurityConfiguration
defaults:
  enforce: "restricted"
  enforce-version: "latest"
  audit: "restricted"
  audit-version: "latest"
  warn: "restricted"
  warn-version: "latest"
exemptions:
  namespaces:
    - "kube-system"
    - "my-privileged-ns"
  usernames:
    - "system:serviceaccount:kube-system:some-sa"
  runtimeClassNames:
    - "kata"
`,
							),
						),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.PodSecurityAdmission{
						Enforce: v1alpha1.PodSecurityStandardRestricted,
						Audit:   v1alpha1.PodSecurityStandardRestricted,
						Warn:    v1alpha1.PodSecurityStandardRestricted,
						Exemptions: v1alpha1.PodSecurityExemptions{
							Namespaces:        []string{"kube-system", "my-privileged-ns"},
							Usernames:         []string{"system:serviceaccount:kube-system:some-sa"},
							RuntimeClassNames: []string{"kata"},
						},
					},
					VariableName,
				),
			},
		},
	}

	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
