// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package taints

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate taints patches for Control Plane", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler(
			"", helpers.TestEnv.Client, NewControlPlanePatch(),
		).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "taints for control plane set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					[]v1alpha1.Taint{{
						Key:    "key",
						Effect: v1alpha1.TaintEffectNoExecute,
						Value:  "value",
					}},
					v1alpha1.ControlPlaneConfigVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/taints",
				ValueMatcher: gomega.ConsistOf(
					map[string]interface{}{"key": "key", "effect": "NoExecute", "value": "value"},
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/taints",
				ValueMatcher: gomega.ConsistOf(
					map[string]interface{}{"key": "key", "effect": "NoExecute", "value": "value"},
				),
			}},
		},
		{
			Name: "taints for control plane set to empty slice to remove default taints",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					[]v1alpha1.Taint{},
					v1alpha1.ControlPlaneConfigVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/taints",
				ValueMatcher: gomega.SatisfyAll(
					gomega.Not(gomega.BeNil()),
					gomega.BeEmpty(),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/taints",
				ValueMatcher: gomega.SatisfyAll(
					gomega.Not(gomega.BeNil()),
					gomega.BeEmpty(),
				),
			}},
		},
	}

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
		})
	}
})
