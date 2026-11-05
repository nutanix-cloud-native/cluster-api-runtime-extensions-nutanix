// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestAuthenticationPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Authentication mutator suite")
}

var _ = Describe("Generate Authentication patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler(
			"", helpers.TestEnv.Client, NewPatch(),
		).(mutation.GeneratePatches)
	}

	// helpers to build variables.
	singleIssuerAuth := func() v1alpha1.Authentication {
		return v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"k8s"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
						Groups:   &v1alpha1.PrefixedClaimOrExpression{Claim: "groups"},
					},
				},
			},
		}
	}

	twoIssuersAuth := func() v1alpha1.Authentication {
		return v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer-a.example.com",
						Audiences: []string{"aud-a"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
					},
				},
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer-b.example.com",
						Audiences: []string{"aud-b-1", "aud-b-2"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"},
					},
				},
			},
		}
	}

	// Matchers for happy path: file contains correct YAML via ContainElements.
	authFileMatcher := func(expectedContentChecker gomega.OmegaMatcher) gomega.OmegaMatcher {
		return gomega.SatisfyAll(
			gomega.HaveKeyWithValue("path", AuthenticationConfigPath),
			gomega.HaveKeyWithValue("permissions", "0600"),
			gomega.HaveKeyWithValue("content", expectedContentChecker),
		)
	}

	filesMatcherWith := func(fileMatchers ...any) capitest.JSONPatchMatcher {
		return capitest.JSONPatchMatcher{
			Operation:    "add",
			Path:         "/spec/template/spec/kubeadmConfigSpec/files",
			ValueMatcher: gomega.ContainElements(fileMatchers...),
		}
	}

	clusterConfigMatcher := func(extraArgsMatcher, extraVolumesMatcher gomega.OmegaMatcher) capitest.JSONPatchMatcher {
		// extraArgs and extraVolumes live under clusterConfiguration.apiServer
		// JSONPatchMatcher can match at clusterConfiguration level; compose via HaveKeyWithValue nesting.
		return capitest.JSONPatchMatcher{
			Operation: "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
			ValueMatcher: gomega.SatisfyAll(
				gomega.HaveKeyWithValue("apiServer", gomega.SatisfyAll(
					extraArgsMatcher,
					extraVolumesMatcher,
				)),
			),
		}
	}

	authExtraArgsMatcher := gomega.HaveKeyWithValue("extraArgs", gomega.ContainElement(
		gomega.SatisfyAll(
			gomega.HaveKeyWithValue("name", AuthenticationConfigArg),
			gomega.HaveKeyWithValue("value", AuthenticationConfigPath),
		),
	))

	authExtraVolumesMatcher := gomega.HaveKeyWithValue("extraVolumes", gomega.ContainElement(
		gomega.SatisfyAll(
			gomega.HaveKeyWithValue("name", authenticationConfigVolumeName),
			gomega.HaveKeyWithValue("hostPath", AuthenticationConfigPath),
			gomega.HaveKeyWithValue("mountPath", AuthenticationConfigPath),
		),
	))

	// Helper: verify YAML content decodes to AuthenticationConfiguration with expected issuers order.
	yamlHasIssuersInOrder := func(expectedURLs ...string) gomega.OmegaMatcher {
		return gomega.WithTransform(func(content string) []string {
			var parsed map[string]any
			_ = yaml.Unmarshal([]byte(content), &parsed)
			jwt, _ := parsed["jwt"].([]any)
			urls := make([]string, 0, len(jwt))
			for _, e := range jwt {
				m, _ := e.(map[string]any)
				iss, _ := m["issuer"].(map[string]any)
				u, _ := iss["url"].(string)
				urls = append(urls, u)
			}
			return urls
		}, gomega.Equal(expectedURLs))
	}

	yamlHasAudiences := func(expected []string) gomega.OmegaMatcher {
		return gomega.WithTransform(func(content string) []string {
			var parsed map[string]any
			_ = yaml.Unmarshal([]byte(content), &parsed)
			jwt, _ := parsed["jwt"].([]any)
			if len(jwt) == 0 {
				return nil
			}
			m, _ := jwt[0].(map[string]any)
			iss, _ := m["issuer"].(map[string]any)
			auds, _ := iss["audiences"].([]any)
			out := make([]string, 0, len(auds))
			for _, a := range auds {
				out = append(out, a.(string))
			}
			return out
		}, gomega.Equal(expected))
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name:                  "omitted variable results in no patches",
			RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
		},
		{
			Name:        "single issuer renders file+volume+flag with correct YAML",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					singleIssuerAuth(),
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				filesMatcherWith(authFileMatcher(gomega.SatisfyAll(
					yamlHasIssuersInOrder("https://issuer.example.com"),
					yamlHasAudiences([]string{"k8s"}),
					// apiVersion/kind present
					gomega.ContainSubstring("apiserver.config.k8s.io/v1"),
					gomega.ContainSubstring("AuthenticationConfiguration"),
				))),
				clusterConfigMatcher(authExtraArgsMatcher, authExtraVolumesMatcher),
			},
		},
		{
			Name:        "two issuers preserves order",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					twoIssuersAuth(),
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				filesMatcherWith(authFileMatcher(
					yamlHasIssuersInOrder(
						"https://issuer-a.example.com",
						"https://issuer-b.example.com",
					),
				)),
				clusterConfigMatcher(authExtraArgsMatcher, authExtraVolumesMatcher),
			},
		},
		{
			Name: "removal: file+volume+flag removed when variable empty",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithFiles(bootstrapv1.File{
					Path:        AuthenticationConfigPath,
					Permissions: "0600",
					Content:     "apiVersion: apiserver.config.k8s.io/v1\nkind: AuthenticationConfiguration\njwt: []\n",
				}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: AuthenticationConfigArg, Value: func() *string { s := AuthenticationConfigPath; return &s }()},
				}).
				NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.Authentication{JWT: []v1alpha1.JWTAuthenticator{}},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "remove",
					Path:         "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraArgs",
					ValueMatcher: gomega.BeNil(),
				},
				{
					Operation:    "remove",
					Path:         "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.BeNil(),
				},
			},
		},
		{
			Name:        "legacy oidc flag conflict returns error",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).WithAPIServerExtraArgs([]bootstrapv1.Arg{{Name: "oidc-issuer-url", Value: func() *string { s := "https://old.example.com"; return &s }()}}).NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					singleIssuerAuth(),
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name:        "existing authentication-config with different path conflicts",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).WithAPIServerExtraArgs([]bootstrapv1.Arg{{Name: AuthenticationConfigArg, Value: func() *string { s := "/other/path.yaml"; return &s }()}}).NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					singleIssuerAuth(),
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name:        "coexistence with admissionconfiguration neither clobbers the other",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).WithFiles(bootstrapv1.File{Path: "/etc/kubernetes/admission.yaml", Permissions: "0600", Content: "apiVersion: apiserver.config.k8s.io/v1\nkind: AdmissionConfiguration\nplugins:\n- name: PodSecurity\n  path: /etc/kubernetes/pod-security-admission.yaml\n"}).WithAPIServerExtraArgs([]bootstrapv1.Arg{{Name: "admission-control-config-file", Value: func() *string { s := "/etc/kubernetes/admission.yaml"; return &s }()}}).NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					singleIssuerAuth(),
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files/1",
					ValueMatcher: authFileMatcher(gomega.ContainSubstring("AuthenticationConfiguration")),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraVolumes",
					ValueMatcher: gomega.ContainElement(gomega.SatisfyAll(
						gomega.HaveKeyWithValue("name", authenticationConfigVolumeName),
						gomega.HaveKeyWithValue("hostPath", AuthenticationConfigPath),
						gomega.HaveKeyWithValue("mountPath", AuthenticationConfigPath),
					)),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraArgs/1",
					ValueMatcher: gomega.SatisfyAll(
						gomega.HaveKeyWithValue("name", AuthenticationConfigArg),
						gomega.HaveKeyWithValue("value", AuthenticationConfigPath),
					),
				},
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
