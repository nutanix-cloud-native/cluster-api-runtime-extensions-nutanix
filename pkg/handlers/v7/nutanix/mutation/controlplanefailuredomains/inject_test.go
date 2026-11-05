// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanefailuredomains

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestControlPlaneFailureDomainsPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane nutanix failuredomains suite")
}

var _ = Describe("Generate ControlPlane nutanix failuredomains patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "FailureDomains set to valid values",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					[]string{"fd-1", "fd-2", "fd-3"},
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.NutanixVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewNutanixClusterTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/controlPlaneFailureDomains",
					ValueMatcher: gomega.ConsistOf(
						gomega.HaveKeyWithValue("name", "fd-1"),
						gomega.HaveKeyWithValue("name", "fd-2"),
						gomega.HaveKeyWithValue("name", "fd-3"),
					),
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
