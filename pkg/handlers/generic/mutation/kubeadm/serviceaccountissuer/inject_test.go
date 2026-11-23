// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serviceaccountissuer

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
			Name:        "issuer only sets service-account-issuer and no other flags",
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
							gomega.ConsistOf(
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
			Name:        "issuer with single audience sets service-account-issuer and api-audiences",
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
							gomega.ConsistOf(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
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
			Name:        "issuer with multiple audiences sorted alphabetically",
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
							gomega.ConsistOf(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
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
							gomega.ConsistOf(
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
			Name:        "issuer with jwksURI including query string sets service-account-jwks-uri",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:  "https://oidc.example.com",
						JWKSURI: "https://oidc.example.com/.well-known/jwks.json?appid=123",
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
							gomega.ConsistOf(
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountIssuerArg),
									gomega.HaveKeyWithValue("value", "https://oidc.example.com"),
								),
								gomega.SatisfyAll(
									gomega.HaveKeyWithValue("name", ServiceAccountJWKSURIArg),
									gomega.HaveKeyWithValue(
										"value",
										"https://oidc.example.com/.well-known/jwks.json?appid=123",
									),
								),
							),
						),
					),
				},
			},
		},
		{
			Name:        "issuer with audiences and jwksURI sets exact three flags",
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
							gomega.ConsistOf(
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
							gomega.ConsistOf(
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
			Name:        "issuer with all flags sets exact four flags",
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
							gomega.ConsistOf(
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
			Name: "idempotent when existing extraArgs already match desired values",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: ServiceAccountIssuerArg, Value: new("https://oidc.example.com")},
					{Name: APIAudiencesArg, Value: new("k8s.example.com,sts.amazonaws.com")},
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
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
		},
		{
			Name: "conflict with existing service-account-max-token-expiration extraArg when unset",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: ServiceAccountMaxTokenExpArg, Value: new("1h0m0s")},
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
			Name: "conflict with existing service-account-issuer extraArg with different value",
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
			Name: "conflict with existing service-account-issuer extraArg with nil value",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: ServiceAccountIssuerArg, Value: nil},
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
			Name: "conflict with existing api-audiences extraArg when unset",
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
			Name: "conflict with existing service-account-jwks-uri extraArg",
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
			Name:        "invalid issuer URL rejected by upstream webhook URL validator",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer: "https://:443",
					},
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name:        "audience with comma rejected by pflag CSV round-trip validator",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:    "https://oidc.example.com",
						Audiences: []string{"aud-a,aud-b"},
					},
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name:        "audience with double quote rejected by pflag CSV parser",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:    "https://oidc.example.com",
						Audiences: []string{`aud"bad`},
					},
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name:        "maxTokenExpiration below 1h rejected by handler validation",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.ServiceAccountIssuer{
						Issuer:             "https://oidc.example.com",
						MaxTokenExpiration: &metav1.Duration{Duration: 30 * time.Minute},
					},
					VariableName,
				),
			},
			ExpectedFailure: true,
		},
		{
			Name:        "overlapping authentication.jwt[].issuer.url and serviceAccountIssuer.issuer rejected",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.KubeadmClusterConfigSpec{
						Authentication: &v1alpha1.Authentication{
							JWT: []v1alpha1.JWTAuthenticator{
								{
									Issuer: v1alpha1.Issuer{
										URL:       "https://same-issuer.example.com",
										Audiences: []string{"k8s"},
									},
									ClaimMappings: v1alpha1.ClaimMappings{
										Username: v1alpha1.PrefixedClaimOrExpression{
											Claim:  "sub",
											Prefix: new(""),
										},
									},
								},
							},
						},
						ServiceAccountIssuer: &v1alpha1.ServiceAccountIssuer{
							Issuer: "https://same-issuer.example.com",
						},
					},
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

	It("is idempotent when Mutate is invoked twice on the same KubeadmControlPlaneTemplate", func() {
		g := gomega.NewWithT(GinkgoT())
		handler := NewPatch()
		item := request.NewKubeadmControlPlaneTemplateRequestItem("test-uid")
		var u unstructured.Unstructured
		g.Expect(u.UnmarshalJSON(item.Object.Raw)).To(gomega.Succeed())

		variable := capitest.VariableWithValue(
			v1alpha1.ClusterConfigVariableName,
			v1alpha1.ServiceAccountIssuer{
				Issuer:             "https://oidc.example.com",
				Audiences:          []string{"sts.amazonaws.com", "k8s.example.com"},
				JWKSURI:            "https://oidc.example.com/jwks?appid=1",
				MaxTokenExpiration: &metav1.Duration{Duration: 2 * time.Hour},
			},
			VariableName,
		)
		vars := map[string]apiextensionsv1.JSON{
			variable.Name: variable.Value,
		}

		g.Expect(handler.Mutate(context.Background(), &u, vars, item.HolderReference, client.ObjectKey{}, nil)).
			To(gomega.Succeed())
		g.Expect(handler.Mutate(context.Background(), &u, vars, item.HolderReference, client.ObjectKey{}, nil)).
			To(gomega.Succeed())

		var kcpt controlplanev1.KubeadmControlPlaneTemplate
		g.Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &kcpt)).To(gomega.Succeed())
		g.Expect(kcpt.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs).
			To(gomega.HaveLen(4))
	})
})
