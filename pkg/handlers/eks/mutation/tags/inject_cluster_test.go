// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tags

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	capav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/testutils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate EKS Tags patches for managed control plane", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewClusterPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "additionalTags set for managed control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					capav1.Tags{
						"Environment": "production",
						"Team":        "platform",
					},
					v1alpha1.EKSVariableName,
					VariableName,
				),
			},
			RequestItem: testutils.NewEKSControlPlaneRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/additionalTags",
				ValueMatcher: gomega.And(
					gomega.HaveKeyWithValue("Environment", "production"),
					gomega.HaveKeyWithValue("Team", "platform"),
				),
			}},
		},
		{
			Name: "empty additionalTags for managed control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					capav1.Tags{},
					v1alpha1.EKSVariableName,
					VariableName,
				),
			},
			RequestItem:           testutils.NewEKSControlPlaneRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
		},
		{
			Name: "additionalTags with special characters for managed control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					capav1.Tags{
						"kubernetes.io/cluster/test-cluster": "owned",
						"Cost-Center":                        "12345",
						"Environment":                        "dev-test",
					},
					v1alpha1.EKSVariableName,
					VariableName,
				),
			},
			RequestItem: testutils.NewEKSControlPlaneRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/additionalTags",
				ValueMatcher: gomega.And(
					gomega.HaveKeyWithValue("kubernetes.io/cluster/test-cluster", "owned"),
					gomega.HaveKeyWithValue("Cost-Center", "12345"),
					gomega.HaveKeyWithValue("Environment", "dev-test"),
				),
			}},
		},
	}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
		})
	}
})
