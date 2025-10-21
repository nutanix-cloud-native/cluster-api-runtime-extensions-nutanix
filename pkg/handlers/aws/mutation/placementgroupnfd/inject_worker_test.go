// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package placementgroupnfd

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

var _ = Describe("Generate AWS Placement Group NFD patches for Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewWorkerPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "placement group set for workers",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.PlacementGroup{
						Name: "test-placement-group",
					},
					v1alpha1.AWSVariableName,
					"placementGroup",
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					map[string]any{
						"machineDeployment": map[string]any{
							"class": "a-worker",
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ContainElement(gomega.HaveKeyWithValue(
					"path", PlacementGroupDiscoveryScriptFileOnRemote,
				)),
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
