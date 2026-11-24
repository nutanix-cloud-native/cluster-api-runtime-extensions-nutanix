// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
)

var _ = Describe("AuthenticationValidator", func() {
	var (
		decoder admission.Decoder
		scheme  *runtime.Scheme
	)

	validAuth := func() *v1alpha1.Authentication {
		return &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"k8s"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{
							Claim:  "email",
							Prefix: new(""),
						},
					},
				},
			},
		}
	}

	buildClusterWithSAIssuer := func(
		version string,
		auth *v1alpha1.Authentication,
		sai *v1alpha1.ServiceAccountIssuer,
	) *clusterv1.Cluster {
		var clusterVars []clusterv1.ClusterVariable
		if auth != nil || sai != nil {
			raw, err := json.Marshal(&variables.ClusterConfigSpec{
				KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
					Authentication:       auth,
					ServiceAccountIssuer: sai,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			clusterVars = append(clusterVars, clusterv1.ClusterVariable{
				Name:  v1alpha1.ClusterConfigVariableName,
				Value: apiextensionsv1.JSON{Raw: raw},
			})
		}

		return &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-auth-cluster",
				Namespace: "test-namespace",
			},
			Spec: clusterv1.ClusterSpec{
				Topology: clusterv1.Topology{
					ClassRef: clusterv1.ClusterClassRef{
						Name: "test-class",
					},
					Version:   version,
					Variables: clusterVars,
				},
			},
		}
	}

	buildCluster := func(version string, auth *v1alpha1.Authentication) *clusterv1.Cluster {
		return buildClusterWithSAIssuer(version, auth, nil)
	}

	buildClassAndTemplate := func(
		classNamespace string,
		extraArgs []bootstrapv1.Arg,
		extraVolumes []bootstrapv1.HostPathMount,
	) (*clusterv1.ClusterClass, *controlplanev1.KubeadmControlPlaneTemplate) {
		cc := &clusterv1.ClusterClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-class",
				Namespace: classNamespace,
			},
			Spec: clusterv1.ClusterClassSpec{
				ControlPlane: clusterv1.ControlPlaneClass{
					TemplateRef: clusterv1.ClusterClassTemplateReference{
						APIVersion: controlplanev1.GroupVersion.String(),
						Kind:       "KubeadmControlPlaneTemplate",
						Name:       "test-kcp-template",
					},
				},
			},
		}
		cpTemplate := &controlplanev1.KubeadmControlPlaneTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-kcp-template",
				Namespace: classNamespace,
			},
			Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
				Template: controlplanev1.KubeadmControlPlaneTemplateResource{
					Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
						KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
							ClusterConfiguration: bootstrapv1.ClusterConfiguration{
								APIServer: bootstrapv1.APIServer{
									ExtraArgs:    extraArgs,
									ExtraVolumes: extraVolumes,
								},
							},
						},
					},
				},
			},
		}
		return cc, cpTemplate
	}

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(controlplanev1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())

		decoder = admission.NewDecoder(scheme)
	})

	It("allows DELETE operations without decoding", func() {
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Delete,
			},
		})
		Expect(resp.Allowed).To(BeTrue())
	})

	It("allows pre-1.34 clusters when authentication is unset", func() {
		cluster := buildCluster("v1.33.5", nil)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
	})

	It("accepts valid authentication on Kubernetes >= 1.34 when ClusterClass is not yet created", func() {
		cluster := buildCluster("v1.34.0", validAuth())
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
	})

	It("rejects authentication on Kubernetes < 1.34", func() {
		cluster := buildCluster("v1.33.9", validAuth())
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("requires Kubernetes >= 1.34.0"))
	})

	It("rejects invalid CEL expression in authentication configuration", func() {
		auth := validAuth()
		auth.JWT[0].ClaimValidationRules = []v1alpha1.ClaimValidationRule{
			{Expression: "claims.exp - (("},
		}
		cluster := buildCluster("v1.35.0", auth)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("Syntax error"))
	})

	It("rejects malformed PEM in certificateAuthority", func() {
		auth := validAuth()
		auth.JWT[0].Issuer.CertificateAuthority = "not-a-valid-pem"
		cluster := buildCluster("v1.35.0", auth)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("certificateAuthority"))
	})

	It("rejects issuer URL overlapping default service-account-issuer", func() {
		auth := validAuth()
		auth.JWT[0].Issuer.URL = "https://kubernetes.default.svc.cluster.local"
		cluster := buildCluster("v1.35.0", auth)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("URL must not overlap with disallowed issuers"))
	})

	It("rejects issuer URL overlapping custom Cluster serviceDomain service-account-issuer", func() {
		auth := validAuth()
		auth.JWT[0].Issuer.URL = "https://kubernetes.default.svc.corp.local"
		cluster := buildCluster("v1.35.0", auth)
		cluster.Spec.ClusterNetwork.ServiceDomain = "corp.local"
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("URL must not overlap with disallowed issuers"))
	})

	It("rejects issuer URL overlapping --service-account-issuer on KubeadmControlPlaneTemplate", func() {
		cc, cpTemplate := buildClassAndTemplate(
			"shared-classes",
			[]bootstrapv1.Arg{
				{Name: authentication.ServiceAccountIssuerArg, Value: new("https://issuer.example.com")},
			},
			nil,
		)
		cluster := buildCluster("v1.35.0", validAuth())
		cluster.Spec.Topology.ClassRef.Namespace = "shared-classes"

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cc, cpTemplate).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("URL must not overlap with disallowed issuers"))
	})

	It("rejects issuer URL overlapping clusterConfig.serviceAccountIssuer.issuer", func() {
		cluster := buildClusterWithSAIssuer(
			"v1.35.0",
			validAuth(),
			&v1alpha1.ServiceAccountIssuer{Issuer: "https://issuer.example.com"},
		)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("URL must not overlap with disallowed issuers"))
	})

	It("allows default kubeadm issuer URL when clusterConfig.serviceAccountIssuer overrides issuer", func() {
		auth := validAuth()
		auth.JWT[0].Issuer.URL = "https://kubernetes.default.svc.cluster.local"
		cluster := buildClusterWithSAIssuer(
			"v1.35.0",
			auth,
			&v1alpha1.ServiceAccountIssuer{Issuer: "https://sa.example.com"},
		)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
	})

	It("rejects Cluster when KubeadmControlPlaneTemplate sets legacy --oidc-* extraArgs", func() {
		cc, cpTemplate := buildClassAndTemplate(
			"test-namespace",
			[]bootstrapv1.Arg{
				{Name: "oidc-issuer-url", Value: new("https://old.example.com")},
			},
			nil,
		)
		cluster := buildCluster("v1.35.0", validAuth())

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cc, cpTemplate).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("--oidc-*"))
	})

	It("rejects Cluster when KubeadmControlPlaneTemplate has conflicting extraVolumes", func() {
		cc, cpTemplate := buildClassAndTemplate(
			"test-namespace",
			nil,
			[]bootstrapv1.HostPathMount{
				{
					Name:      "custom-auth",
					HostPath:  "/etc/kubernetes/custom-auth.yaml",
					MountPath: authentication.AuthenticationConfigPath,
					ReadOnly:  new(true),
					PathType:  corev1.HostPathFile,
				},
			},
		)
		cluster := buildCluster("v1.35.0", validAuth())

		client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cc, cpTemplate).Build()
		v := NewAuthenticationValidator(client, decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeFalse())
		Expect(resp.Result.Message).To(ContainSubstring("extraVolume"))
	})

	It("rejects invalid authentication through the live envtest /validate-cluster webhook", func() {
		ns, err := env.CreateNamespace(ctx, "auth-webhook", nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = env.Cleanup(ctx, ns)
		})

		invalidVersionCluster := buildCluster("v1.33.9", validAuth())
		invalidVersionCluster.Namespace = ns.Name
		err = env.Create(ctx, invalidVersionCluster)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("requires Kubernetes >= 1.34.0"))

		cc, cpTemplate := buildClassAndTemplate(
			ns.Name,
			[]bootstrapv1.Arg{
				{Name: authentication.ServiceAccountIssuerArg, Value: new("https://issuer.example.com")},
			},
			nil,
		)
		cc.Spec.Infrastructure = clusterv1.InfrastructureClass{
			TemplateRef: clusterv1.ClusterClassTemplateReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
				Kind:       "GenericInfrastructureClusterTemplate",
				Name:       "dummy-infra",
			},
		}
		Expect(env.CreateAndWait(ctx, cpTemplate)).To(Succeed())
		Expect(env.CreateAndWait(ctx, cc)).To(Succeed())
		DeferCleanup(func() {
			_ = env.Cleanup(ctx, cc, cpTemplate)
		})

		saConflictCluster := buildCluster("v1.35.0", validAuth())
		saConflictCluster.Namespace = ns.Name
		err = env.Create(ctx, saConflictCluster)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("URL must not overlap with disallowed issuers"))

		nonConflictingAuth := validAuth()
		nonConflictingAuth.JWT[0].Issuer.URL = "https://idp.example.com"
		validCluster := buildCluster("v1.35.0", nonConflictingAuth)
		validCluster.Namespace = ns.Name
		Expect(env.Create(ctx, validCluster)).To(Succeed())
		DeferCleanup(func() {
			_ = env.Cleanup(ctx, validCluster)
		})
	})
})
