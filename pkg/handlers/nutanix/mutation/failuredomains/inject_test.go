// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomains

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestFailureDomainsPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Nutanix FailureDomains suite")
}

var _ = Describe("Generate Nutanix FailureDomains endpoint patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "FailureDomains set to valid information",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NutanixFailureDomains{
						v1alpha1.NutanixFailureDomain{
							Name: "fd1",
							Cluster: v1beta1.NutanixResourceIdentifier{
								Type: v1beta1.NutanixIdentifierName,
								Name: &testPEName,
							},
							Subnets: []v1beta1.NutanixResourceIdentifier{
								{
									Type: v1beta1.NutanixIdentifierName,
									Name: &testSubnetName,
								},
							},
							ControlPlane: true,
						},
					},
					v1alpha1.NutanixVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewNutanixClusterTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "replace",
					Path:         "/spec/template/spec/failureDomains",
					ValueMatcher: gomega.HaveLen(1),
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
