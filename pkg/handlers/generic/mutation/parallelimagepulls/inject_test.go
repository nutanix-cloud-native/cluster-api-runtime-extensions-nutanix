// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package parallelimagepulls

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestMaxParallelImagePullsPerNodePatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "max parallel image pulls mutator suite")
}

var patchGenerator = func() mutation.GeneratePatches {
	return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
}

var _ = DescribeTable("Generate max parallel image pulls patches",
	func(tt capitest.PatchTestDef) {
		capitest.AssertGeneratePatches(
			GinkgoT(),
			patchGenerator,
			&tt,
		)
	},
	Entry("unset max parallel image pulls defaults to 1 with AWS control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.AWSClusterConfigSpec{},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("unset max parallel image pulls defaults to 1 with Nutanix control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.NutanixClusterConfigSpec{},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("unset max parallel image pulls defaults to 1 with Docker control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.DockerClusterConfigSpec{},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("unset max parallel image pulls defaults to 1 with AWS workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.AWSClusterConfigSpec{},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("unset max parallel image pulls defaults to 1 with Nutanix workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.NutanixClusterConfigSpec{},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("unset max parallel image pulls defaults to 1 with Docker workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.DockerClusterConfigSpec{},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 1 with AWS control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.AWSClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(1)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 1 with Nutanix control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.NutanixClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(1)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 1 with Docker control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.DockerClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(1)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 1 with AWS workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.AWSClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(1)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 1 with Nutanix workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.NutanixClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(1)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 1 with Docker workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.DockerClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(1)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElement(
					gomega.HaveKeyWithValue(
						"path",
						"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to unlimited with AWS control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.AWSClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(0)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to unlimited with Nutanix control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.NutanixClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(0)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to unlimited with Docker control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.DockerClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(0)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to unlimited with AWS workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.AWSClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(0)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to unlimited with Nutanix workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.NutanixClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(0)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to unlimited with Docker workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.DockerClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(0)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 10 with AWS control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.AWSClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(10)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
maxParallelImagePulls: 10
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 10 with Nutanix control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.NutanixClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(10)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
maxParallelImagePulls: 10
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 10 with Docker control plane", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.DockerClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(10)),
					},
				},
			),
		},
		RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
maxParallelImagePulls: 10
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 10 with AWS workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.AWSClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(10)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
maxParallelImagePulls: 10
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 10 with Nutanix workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.NutanixClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(10)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
maxParallelImagePulls: 10
`,
						),
					),
				),
			},
		},
	}),
	Entry("max parallel image pulls set to 10 with Docker workers", capitest.PatchTestDef{
		Vars: []runtimehooksv1.Variable{
			capitest.VariableWithValue(
				v1alpha1.ClusterConfigVariableName,
				v1alpha1.DockerClusterConfigSpec{
					KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
						MaxParallelImagePullsPerNode: ptr.To(int32(10)),
					},
				},
			),
			capitest.VariableWithValue(
				runtimehooksv1.BuiltinsName,
				apiextensionsv1.JSON{
					Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
				},
			),
		},
		RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
		ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
			{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ConsistOf(
					gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"path",
							"/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json",
						),
						gomega.HaveKeyWithValue("owner", "root:root"),
						gomega.HaveKeyWithValue("permissions", "0644"),
						gomega.HaveKeyWithValue("content", `---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
serializeImagePulls: false
maxParallelImagePulls: 10
`,
						),
					),
				),
			},
		},
	}),
)
