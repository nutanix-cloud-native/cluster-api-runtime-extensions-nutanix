// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serviceaccountissuer

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestServiceAccountIssuerPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "ServiceAccount issuer mutator suite")
}

var _ = Describe("Generate ServiceAccount issuer patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler(
			"", helpers.TestEnv.Client, NewPatch(),
		).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name:                  "variable not set results in no patches",
			RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
		},
		{
			Name:        "issuer only sets service-account-issuer",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer: "https://oidc.example.com",
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElement(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer with single audience sets api-audiences",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:    "https://oidc.example.com",
						Audiences: []string{"k8s.example.com"},
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElement(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", APIAudiencesArg),
									gomega.HaveKeyWithValue("value", "k8s.example.com"),
								),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer with multiple audiences sorted",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:    "https://oidc.example.com",
						Audiences: []string{"c", "a", "b"},
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElement(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", APIAudiencesArg),
									gomega.HaveKeyWithValue("value", "a,b,c"),
								),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer with audiences sts and k8s sorted",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:    "https://oidc.example.com",
						Audiences: []string{"sts.amazonaws.com", "k8s.example.com"},
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElements(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", APIAudiencesArg),
									gomega.HaveKeyWithValue("value", "k8s.example.com,sts.amazonaws.com"),
								),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer without audiences does not set api-audiences",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer: "https://oidc.example.com",
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElement(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
							),
						),
					),
				},
			},
			UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElement(
								gomega.HaveKeyWithValue("name", APIAudiencesArg),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer with jwksURI sets service-account-jwks-uri",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:  "https://oidc.example.com",
						JWKSURI: "https://oidc.example.com/.well-known/jwks.json",
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElements(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountJWKSURIArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com/.well-known/jwks.json"),
								),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer with audiences and jwksURI sets all three",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:    "https://oidc.example.com",
						Audiences: []string{"sts.amazonaws.com"},
						JWKSURI:   "https://oidc.example.com/.well-known/jwks.json",
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElements(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", APIAudiencesArg),
									gomega.HaveKeyWithValue("value", "sts.amazonaws.com"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountJWKSURIArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com/.well-known/jwks.json"),
								),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer with maxTokenExpiration sets service-account-max-token-expiration",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:             "https://oidc.example.com",
						MaxTokenExpiration: &metav1.Duration{Duration: time.Hour},
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElements(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountMaxTokenExpArg),
									gomega.HaveKeyWithValue("value", "1h0m0s"),
								),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer with all flags sets all four",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:             "https://oidc.example.com",
						Audiences:          []string{"a"},
						JWKSURI:            "https://oidc.example.com/jwks",
						MaxTokenExpiration: &metav1.Duration{Duration: 24 * time.Hour},
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.HaveKeyWithValue(
							"extraArgs",
							gomega.ContainElements(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", APIAudiencesArg),
									gomega.HaveKeyWithValue("value", "a"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountJWKSURIArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com/jwks"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountMaxTokenExpArg),
									gomega.HaveKeyWithValue("value", "24h0m0s"),
								),
							),
						),
					),
				},
			},
		},
		{
			Name: "conflict with existing service-account-max-token-expiration extraArg errors",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: ServiceAccountMaxTokenExpArg, Value: new("1h")},
				}).
				NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer: "https://oidc.example.com",
					},
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name: "conflict with existing service-account-issuer extraArg errors",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: ServiceAccountIssuerArg, Value: new("https://existing.example.com")},
				}).
				NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer: "https://oidc.example.com",
					},
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name: "conflict with existing api-audiences extraArg errors",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: APIAudiencesArg, Value: new("api")},
				}).
				NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer: "https://oidc.example.com",
					},
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name: "conflict with existing service-account-jwks-uri extraArg errors",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: ServiceAccountJWKSURIArg, Value: new("https://jwks.example.com")},
				}).
				NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer: "https://oidc.example.com",
					},
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name:        "worker KubeadmConfigTemplate is ignored",
			RequestItem: request.NewKubeadmConfigTemplateRequest("", "worker-bootstrap"),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer: "https://oidc.example.com",
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
		},
		{
			Name: "coexists with unrelated apiServer extraArgs such as authentication-config",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: "authentication-config", Value: new("/etc/kubernetes/authentication.yaml")},
				}).
				NewRequest(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:    "https://oidc.example.com",
						Audiences: []string{"sts.amazonaws.com", "k8s.example.com"},
					},
					VariableName,
				),
			},
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraArgs/1",
					ValueMatcher: gomega.SatisfyAll(
						gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
						gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraArgs/2",
					ValueMatcher: gomega.SatisfyAll(
						gomega.HaveKeyWithValue("name", APIAudiencesArg),
						gomega.HaveKeyWithValue("value", "k8s.example.com,sts.amazonaws.com"),
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
