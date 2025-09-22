// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeproxymode

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/testutils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestKubeProxyModePatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "kube-proxy mode mutator suite")
}

type testObj struct {
	patchTest capitest.PatchTestDef
	cluster   *clusterv1.Cluster
}

var _ = Describe("Generate kube proxy mode patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		clientScheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
		utilruntime.Must(clusterv1.AddToScheme(clientScheme))
		cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).To(gomega.BeNil())
		return mutation.NewMetaGeneratePatchesHandler("", cl, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []testObj{{
		patchTest: capitest.PatchTestDef{
			Name: "disable kube proxy with AWS",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AWSClusterConfigSpec{},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/kubeadmConfigSpec/initConfiguration/skipPhases",
				ValueMatcher: gomega.ConsistOf("addon/kube-proxy"),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "aws",
				},
			},
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Version: "dummy-version",
					Class:   "dummy-class",
					ControlPlane: clusterv1.ControlPlaneTopology{
						Metadata: clusterv1.ObjectMeta{
							Annotations: map[string]string{
								controlplanev1.SkipKubeProxyAnnotation: "",
							},
						},
					},
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "disable kube proxy with Docker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.DockerClusterConfigSpec{},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/kubeadmConfigSpec/initConfiguration/skipPhases",
				ValueMatcher: gomega.ConsistOf("addon/kube-proxy"),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "docker",
				},
			},
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Version: "dummy-version",
					Class:   "dummy-class",
					ControlPlane: clusterv1.ControlPlaneTopology{
						Metadata: clusterv1.ObjectMeta{
							Annotations: map[string]string{
								controlplanev1.SkipKubeProxyAnnotation: "",
							},
						},
					},
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "disable kube proxy with Nutanix",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NutanixClusterConfigSpec{},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/kubeadmConfigSpec/initConfiguration/skipPhases",
				ValueMatcher: gomega.ConsistOf("addon/kube-proxy"),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "nutanix",
				},
			},
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Version: "dummy-version",
					Class:   "dummy-class",
					ControlPlane: clusterv1.ControlPlaneTopology{
						Metadata: clusterv1.ObjectMeta{
							Annotations: map[string]string{
								controlplanev1.SkipKubeProxyAnnotation: "",
							},
						},
					},
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy iptables mode with Nutanix",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NutanixClusterConfigSpec{
						KubeProxy: &v1alpha1.KubeProxy{
							Mode: v1alpha1.KubeProxyModeIPTables,
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue("path", "/etc/kubernetes/kubeproxy-config.yaml"),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: iptables
`,
						),
					),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
				ValueMatcher: gomega.ConsistOf(
					`/bin/sh -ec '(grep -q "^kind: KubeProxyConfiguration$" /run/kubeadm/kubeadm.yaml && sed -i -e "s/^\(kind: KubeProxyConfiguration\)$/\1\nmode: iptables/" /run/kubeadm/kubeadm.yaml) || (cat /etc/kubernetes/kubeproxy-config.yaml >>/run/kubeadm/kubeadm.yaml)'`, //nolint:lll // Just a long command.
				),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "nutanix",
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy iptables mode with AWS",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AWSClusterConfigSpec{
						KubeProxy: &v1alpha1.KubeProxy{
							Mode: v1alpha1.KubeProxyModeIPTables,
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue("path", "/etc/kubernetes/kubeproxy-config.yaml"),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: iptables
`,
						),
					),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
				ValueMatcher: gomega.ConsistOf(
					`/bin/sh -ec '(grep -q "^kind: KubeProxyConfiguration$" /run/kubeadm/kubeadm.yaml && sed -i -e "s/^\(kind: KubeProxyConfiguration\)$/\1\nmode: iptables/" /run/kubeadm/kubeadm.yaml) || (cat /etc/kubernetes/kubeproxy-config.yaml >>/run/kubeadm/kubeadm.yaml)'`, //nolint:lll // Just a long command.
				),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "aws",
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy iptables mode with Docker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.DockerClusterConfigSpec{
						KubeProxy: &v1alpha1.KubeProxy{
							Mode: v1alpha1.KubeProxyModeIPTables,
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue("path", "/etc/kubernetes/kubeproxy-config.yaml"),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: iptables
`,
						),
					),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
				ValueMatcher: gomega.ConsistOf(
					`/bin/sh -ec '(grep -q "^kind: KubeProxyConfiguration$" /run/kubeadm/kubeadm.yaml && sed -i -e "s/^\(kind: KubeProxyConfiguration\)$/\1\nmode: iptables/" /run/kubeadm/kubeadm.yaml) || (cat /etc/kubernetes/kubeproxy-config.yaml >>/run/kubeadm/kubeadm.yaml)'`, //nolint:lll // Just a long command.
				),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "docker",
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy nftables mode with Nutanix",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NutanixClusterConfigSpec{
						KubeProxy: &v1alpha1.KubeProxy{
							Mode: v1alpha1.KubeProxyModeNFTables,
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue("path", "/etc/kubernetes/kubeproxy-config.yaml"),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: nftables
`,
						),
					),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
				ValueMatcher: gomega.ConsistOf(
					`/bin/sh -ec '(grep -q "^kind: KubeProxyConfiguration$" /run/kubeadm/kubeadm.yaml && sed -i -e "s/^\(kind: KubeProxyConfiguration\)$/\1\nmode: nftables/" /run/kubeadm/kubeadm.yaml) || (cat /etc/kubernetes/kubeproxy-config.yaml >>/run/kubeadm/kubeadm.yaml)'`, //nolint:lll // Just a long command.
				),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "nutanix",
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy nftables mode with AWS",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AWSClusterConfigSpec{
						KubeProxy: &v1alpha1.KubeProxy{
							Mode: v1alpha1.KubeProxyModeNFTables,
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue("path", "/etc/kubernetes/kubeproxy-config.yaml"),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: nftables
`,
						),
					),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
				ValueMatcher: gomega.ConsistOf(
					`/bin/sh -ec '(grep -q "^kind: KubeProxyConfiguration$" /run/kubeadm/kubeadm.yaml && sed -i -e "s/^\(kind: KubeProxyConfiguration\)$/\1\nmode: nftables/" /run/kubeadm/kubeadm.yaml) || (cat /etc/kubernetes/kubeproxy-config.yaml >>/run/kubeadm/kubeadm.yaml)'`, //nolint:lll // Just a long command.
				),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "aws",
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy nftables mode with Docker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.DockerClusterConfigSpec{
						KubeProxy: &v1alpha1.KubeProxy{
							Mode: v1alpha1.KubeProxyModeNFTables,
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue("path", "/etc/kubernetes/kubeproxy-config.yaml"),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: nftables
`,
						),
					),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
				ValueMatcher: gomega.ConsistOf(
					`/bin/sh -ec '(grep -q "^kind: KubeProxyConfiguration$" /run/kubeadm/kubeadm.yaml && sed -i -e "s/^\(kind: KubeProxyConfiguration\)$/\1\nmode: nftables/" /run/kubeadm/kubeadm.yaml) || (cat /etc/kubernetes/kubeproxy-config.yaml >>/run/kubeadm/kubeadm.yaml)'`, //nolint:lll // Just a long command.
				),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "docker",
				},
			},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "disable kube proxy with EKS",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.EKSClusterConfigSpec{},
				),
			},
			RequestItem: testutils.NewEKSControlPlaneRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/kubeProxy/disable",
				ValueMatcher: gomega.Equal(true),
			}},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: request.Namespace,
				Labels: map[string]string{
					clusterv1.ProviderNameLabel: "eks",
				},
			},
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Version: "dummy-version",
					Class:   "dummy-class",
					ControlPlane: clusterv1.ControlPlaneTopology{
						Metadata: clusterv1.ObjectMeta{
							Annotations: map[string]string{
								controlplanev1.SkipKubeProxyAnnotation: "",
							},
						},
					},
				},
			},
		},
	}}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.patchTest.Name, func() {
			if tt.cluster != nil {
				clientScheme := runtime.NewScheme()
				utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
				utilruntime.Must(clusterv1.AddToScheme(clientScheme))
				cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect(cl.Create(context.Background(), tt.cluster)).To(gomega.Succeed())
				DeferCleanup(cl.Delete, context.Background(), tt.cluster)
			}
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt.patchTest)
		})
	}
})
