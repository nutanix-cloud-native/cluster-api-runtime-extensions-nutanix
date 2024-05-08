// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package securitygroups

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/workerconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate AWS SecurityGroups patches for Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewWorkerPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "SecurityGroups for workers set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					workerconfig.MetaVariableName,
					v1alpha1.AdditionalSecurityGroup{
						{ID: "sg-1"},
						{ID: "sg-2"},
						{ID: "sg-3"},
					},
					v1alpha1.AWSVariableName,
					VariableName,
				),
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "add",
					Path:         "/spec/template/spec/additionalSecurityGroups",
					ValueMatcher: gomega.HaveLen(3),
				},
				// TODO(shalinpatel): add matcher to check if all SG are set
				// {
				// 	Operation: "add",
				// 	Path:      "/spec/template/spec/additionalSecurityGroups",
				// 	ValueMatcher: gomega.ContainElements(
				// 		gomega.HaveKeyWithValue("id", "sg-1"),
				// 		gomega.HaveKeyWithValue("id", "sg-2"),
				// 		gomega.HaveKeyWithValue("id", "sg-3"),
				// 	),
				// },
			},
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
