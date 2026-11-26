// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type warningCollector struct {
	mu       sync.Mutex
	warnings []string
}

func (w *warningCollector) HandleWarningHeader(_ int, _, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.warnings = append(w.warnings, text)
}

func (w *warningCollector) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.warnings = nil
}

func (w *warningCollector) Warnings() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.warnings))
	copy(out, w.warnings)
	return out
}

var _ = Describe("ServiceAccountIssuerValidator", func() {
	var (
		decoder admission.Decoder
		scheme  *runtime.Scheme
	)

	buildSAICluster := func(
		sai *v1alpha1.ServiceAccountIssuer,
		serviceDomain string,
	) *clusterv1.Cluster {
		var clusterVars []clusterv1.ClusterVariable
		if sai != nil {
			raw, err := json.Marshal(&variables.ClusterConfigSpec{
				KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
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
				Name:      "test-sai-cluster",
				Namespace: "test-namespace",
			},
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: clusterv1.ClusterNetwork{
					ServiceDomain: serviceDomain,
				},
				Topology: clusterv1.Topology{
					ClassRef: clusterv1.ClusterClassRef{
						Name: "test-class",
					},
					Version:   "v1.35.0",
					Variables: clusterVars,
				},
			},
		}
	}

	buildUpdateRequest := func(oldCluster, newCluster *clusterv1.Cluster) admission.Request {
		oldRaw, err := json.Marshal(oldCluster)
		Expect(err).NotTo(HaveOccurred())
		newRaw, err := json.Marshal(newCluster)
		Expect(err).NotTo(HaveOccurred())

		return admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				Object:    runtime.RawExtension{Raw: newRaw},
				OldObject: runtime.RawExtension{Raw: oldRaw},
			},
		}
	}

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())

		decoder = admission.NewDecoder(scheme)
	})

	It("allows DELETE operations without decoding", func() {
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Delete,
			},
		})
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("allows non-topology clusters without warnings", func() {
		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "non-topology",
				Namespace: "test-namespace",
			},
		}
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("allows clusters without serviceAccountIssuer without warnings", func() {
		cluster := buildSAICluster(nil, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("allows serviceAccountIssuer with issuer only without warnings", func() {
		cluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://oidc.example.com",
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("allows CREATE without warnings when audiences includes serviceAccountIssuer.issuer", func() {
		cluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"sts.amazonaws.com", "https://oidc.example.com"},
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("allows CREATE without warnings when audiences includes default Kubernetes audience", func() {
		cluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://oidc.example.com",
			Audiences: []string{
				"sts.amazonaws.com",
				"https://kubernetes.default.svc.cluster.local",
			},
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("allows CREATE without warnings when audiences matches custom serviceDomain default audience", func() {
		cluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://oidc.example.com",
			Audiences: []string{
				"sts.amazonaws.com",
				"https://kubernetes.default.svc.corp.local",
			},
		}, "corp.local")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("warns on CREATE when audiences omits both issuer and default Kubernetes audience", func() {
		cluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"sts.amazonaws.com"},
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(HaveLen(1))
		Expect(resp.Warnings[0]).To(ContainSubstring("clusterConfig.serviceAccountIssuer.audiences"))
		Expect(resp.Warnings[0]).To(ContainSubstring("https://oidc.example.com"))
		Expect(resp.Warnings[0]).To(ContainSubstring("https://kubernetes.default.svc.cluster.local"))
	})

	It("warns on CREATE when custom serviceDomain is set but audiences only includes cluster.local", func() {
		cluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://oidc.example.com",
			Audiences: []string{
				"sts.amazonaws.com",
				"https://kubernetes.default.svc.cluster.local",
			},
		}, "corp.local")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), createKubeletAdmissionRequest(cluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(HaveLen(1))
		Expect(resp.Warnings[0]).To(ContainSubstring("https://kubernetes.default.svc.corp.local"))
	})

	It("emits a single deduplicated warning on UPDATE from no serviceAccountIssuer to external-only audiences", func() {
		oldCluster := buildSAICluster(nil, "")
		newCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"sts.amazonaws.com"},
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), buildUpdateRequest(oldCluster, newCluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(HaveLen(1))
		Expect(resp.Warnings[0]).To(ContainSubstring("https://oidc.example.com"))
		Expect(resp.Warnings[0]).To(ContainSubstring("https://kubernetes.default.svc.cluster.local"))
	})

	It("warns on UPDATE when adding audiences drops the previous implicit issuer audience", func() {
		oldCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://oidc.example.com",
		}, "")
		newCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://oidc.example.com",
			Audiences: []string{
				"sts.amazonaws.com",
				"https://kubernetes.default.svc.cluster.local",
			},
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), buildUpdateRequest(oldCluster, newCluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(HaveLen(1))
		Expect(resp.Warnings[0]).To(ContainSubstring("https://oidc.example.com"))
	})

	It("warns on UPDATE when narrowing explicit audiences", func() {
		oldCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"https://oidc.example.com", "sts.amazonaws.com", "vault"},
		}, "")
		newCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"https://oidc.example.com", "sts.amazonaws.com"},
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), buildUpdateRequest(oldCluster, newCluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(HaveLen(1))
		Expect(resp.Warnings[0]).To(ContainSubstring("vault"))
	})

	It("warns on UPDATE when clearing audiences drops previously accepted external audiences", func() {
		oldCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"https://oidc.example.com", "sts.amazonaws.com"},
		}, "")
		newCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://oidc.example.com",
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), buildUpdateRequest(oldCluster, newCluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(HaveLen(1))
		Expect(resp.Warnings[0]).To(ContainSubstring("sts.amazonaws.com"))
	})

	It("does not warn on UPDATE when reordering or expanding audiences", func() {
		oldCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"https://oidc.example.com", "sts.amazonaws.com"},
		}, "")
		newCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"sts.amazonaws.com", "https://oidc.example.com", "vault"},
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), buildUpdateRequest(oldCluster, newCluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("does not emit an audiences warning on UPDATE when rotating issuer without audiences", func() {
		oldCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://old-oidc.example.com",
		}, "")
		newCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://new-oidc.example.com",
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), buildUpdateRequest(oldCluster, newCluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("allows UPDATE that repairs a malformed clusterConfig variable in oldObject", func() {
		oldCluster := buildSAICluster(nil, "")
		oldCluster.Spec.Topology.Variables = []clusterv1.ClusterVariable{
			{
				Name:  v1alpha1.ClusterConfigVariableName,
				Value: apiextensionsv1.JSON{Raw: []byte(`"not-an-object"`)},
			},
		}
		newCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"https://oidc.example.com", "sts.amazonaws.com"},
		}, "")
		v := NewServiceAccountIssuerValidator(decoder)

		resp := v.validate(context.Background(), buildUpdateRequest(oldCluster, newCluster))
		Expect(resp.Allowed).To(BeTrue())
		Expect(resp.Warnings).To(BeEmpty())
	})

	It("delivers admission warnings through the live envtest /validate-cluster webhook", func() {
		ns, err := env.CreateNamespace(ctx, "sai-webhook", nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			_ = env.Cleanup(ctx, ns)
		})

		collector := &warningCollector{}
		cfg := rest.CopyConfig(env.Config)
		cfg.WarningHandler = collector
		warnClient, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: env.GetScheme()})
		Expect(err).NotTo(HaveOccurred())

		validCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://sa-issuer.e2e.caren.nutanix.com",
			Audiences: []string{
				"sts.e2e.caren.nutanix.com",
				"https://kubernetes.default.svc.cluster.local",
			},
		}, "")
		validCluster.Name = "sai-valid-cluster"
		validCluster.Namespace = ns.Name
		Expect(warnClient.Create(ctx, validCluster)).To(Succeed())
		DeferCleanup(func() {
			_ = env.Cleanup(ctx, validCluster)
		})
		Expect(collector.Warnings()).To(BeEmpty())

		collector.Reset()
		narrowedRaw, err := json.Marshal(&variables.ClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				ServiceAccountIssuer: &v1alpha1.ServiceAccountIssuer{
					Issuer: "https://sa-issuer.e2e.caren.nutanix.com",
					Audiences: []string{
						"https://kubernetes.default.svc.cluster.local",
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())
		validCluster.Spec.Topology.Variables = []clusterv1.ClusterVariable{
			{
				Name:  v1alpha1.ClusterConfigVariableName,
				Value: apiextensionsv1.JSON{Raw: narrowedRaw},
			},
		}
		Expect(warnClient.Update(ctx, validCluster)).To(Succeed())
		Expect(collector.Warnings()).To(ContainElement(
			ContainSubstring("sts.e2e.caren.nutanix.com"),
		))

		collector.Reset()
		externalOnlyCluster := buildSAICluster(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"sts.amazonaws.com"},
		}, "")
		externalOnlyCluster.Name = "sai-external-only"
		externalOnlyCluster.Namespace = ns.Name
		Expect(warnClient.Create(ctx, externalOnlyCluster)).To(Succeed())
		DeferCleanup(func() {
			_ = env.Cleanup(ctx, externalOnlyCluster)
		})
		Expect(collector.Warnings()).To(ContainElement(
			SatisfyAll(
				ContainSubstring("https://oidc.example.com"),
				ContainSubstring("https://kubernetes.default.svc.cluster.local"),
			),
		))
	})
})
