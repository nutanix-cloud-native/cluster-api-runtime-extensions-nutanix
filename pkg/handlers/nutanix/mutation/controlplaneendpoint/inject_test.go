// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneendpoint

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestControlPlaneEndpointPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Nutanix ControlPlane endpoint suite")
}

var _ = Describe("Generate Nutanix ControlPlane endpoint patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "ControlPlaneEndpoint set to valid host and port",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
					nutanixclusterconfig.NutanixVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewNutanixClusterTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "replace",
					Path:         "/spec/template/spec/controlPlaneEndpoint/host",
					ValueMatcher: gomega.Equal("10.20.100.10"),
				},
				{
					Operation:    "replace",
					Path:         "/spec/template/spec/controlPlaneEndpoint/port",
					ValueMatcher: gomega.BeEquivalentTo(6443),
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
