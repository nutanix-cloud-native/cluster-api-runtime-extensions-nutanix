// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ami

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate AMI patches for ControlPlane", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("",
			helpers.TestEnv.Client,
			NewControlPlanePatch(),
		).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "AMI set for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AMISpec{ID: "ami-controlplane"},
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewCPAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "add",
					Path:         "/spec/template/spec/ami/id",
					ValueMatcher: gomega.Equal("ami-controlplane"),
				},
			},
		},
		{
			Name: "AMI lookup format set for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AMISpec{
						Lookup: &v1alpha1.AMILookup{
							Format: "test-{{.kubernetesVersion}}-format",
							Org:    "1234",
							BaseOS: "testOS",
						},
					},
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewCPAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "add",
					Path:         "/spec/template/spec/imageLookupFormat",
					ValueMatcher: gomega.Equal("test-{{.kubernetesVersion}}-format"),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/imageLookupOrg",
					ValueMatcher: gomega.Equal("1234"),
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
