// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package securitygroups

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/workerconfig"
)

var _ = Describe("Generate AWS SecurityGroups patches for Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewWorkerPatch()).(mutation.GeneratePatches)
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
						{ID: ptr.To("sg-1")},
						{ID: ptr.To("sg-2")},
						{ID: ptr.To("sg-3")},
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
