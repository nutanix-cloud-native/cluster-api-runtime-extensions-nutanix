// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ami

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/workerconfig"
)

var _ = Describe("Generate AMI patches for Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewWorkerPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "AMI set for workers",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					workerconfig.MetaVariableName,
					v1alpha1.AMISpec{ID: "ami-controlplane"},
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
					Path:         "/spec/template/spec/ami/id",
					ValueMatcher: gomega.Equal("ami-controlplane"),
				},
			},
		},
		{
			Name: "AMI lookup format set for worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					workerconfig.MetaVariableName,
					v1alpha1.AMISpec{
						Lookup: &v1alpha1.AMILookup{
							Format: "test-{{.kubernetesVersion}}-format",
							Org:    "12345",
							BaseOS: "testOS",
						},
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
					Path:         "/spec/template/spec/imageLookupFormat",
					ValueMatcher: gomega.Equal("test-{{.kubernetesVersion}}-format"),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/imageLookupOrg",
					ValueMatcher: gomega.Equal("12345"),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/imageLookupBaseOS",
					ValueMatcher: gomega.Equal("testOS"),
				},
			},
			UnexpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "add",
					Path:         "/spec/template/spec/ami/id",
					ValueMatcher: gomega.Equal(""),
				},
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
