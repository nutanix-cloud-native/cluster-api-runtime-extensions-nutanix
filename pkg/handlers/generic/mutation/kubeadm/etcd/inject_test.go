// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestEtcdPolicyPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "etcd mutator suite")
}

var _ = Describe("Generate etcd patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "etcd imageRepository and imageTag set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Repository: "my-registry.io/my-org/my-repo",
							Tag:        "v3.5.99_custom.0",
						},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"etcd",
						gomega.HaveKeyWithValue(
							"local",
							gomega.And(
								gomega.HaveKeyWithValue("imageRepository", "my-registry.io/my-org/my-repo"),
								gomega.HaveKeyWithValue("imageTag", "v3.5.99_custom.0"),
								gomega.HaveKeyWithValue("extraArgs", gomega.ContainElements(
									gomega.HaveKeyWithValue("name", "auto-tls"),
									gomega.HaveKeyWithValue("name", "peer-auto-tls"),
									gomega.HaveKeyWithValue("name", "cipher-suites"),
									gomega.HaveKeyWithValue("name", "tls-min-version"),
								)),
							),
						),
					),
				},
			},
		},
		{
			Name: "etcd imageRepository set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Repository: "my-registry.io/my-org/my-repo",
						},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"etcd",
						gomega.HaveKeyWithValue(
							"local",
							gomega.And(
								gomega.HaveKeyWithValue("imageRepository", "my-registry.io/my-org/my-repo"),
								gomega.HaveKeyWithValue("extraArgs", gomega.ContainElements(
									gomega.HaveKeyWithValue("name", "auto-tls"),
									gomega.HaveKeyWithValue("name", "peer-auto-tls"),
									gomega.HaveKeyWithValue("name", "cipher-suites"),
									gomega.HaveKeyWithValue("name", "tls-min-version"),
								)),
							),
						),
					),
				},
			},
		},
		{
			Name: "etcd imageTag set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Tag: "v3.5.99_custom.0",
						},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"etcd",
						gomega.HaveKeyWithValue(
							"local",
							gomega.And(
								gomega.HaveKeyWithValue("imageTag", "v3.5.99_custom.0"),
								gomega.HaveKeyWithValue("extraArgs", gomega.ContainElements(
									gomega.HaveKeyWithValue("name", "auto-tls"),
									gomega.HaveKeyWithValue("name", "peer-auto-tls"),
									gomega.HaveKeyWithValue("name", "cipher-suites"),
									gomega.HaveKeyWithValue("name", "tls-min-version"),
								)),
							),
						),
					),
				},
			},
		},
	}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
