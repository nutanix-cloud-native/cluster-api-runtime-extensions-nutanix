// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	dockerclusterconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

var _ = Describe("Docker CustomImage patches for ControlPlane", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewControlPlanePatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "image unset for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{Raw: []byte(`{"controlPlane": {"version": "v1.2.3"}}`)},
				),
			},
			RequestItem: request.NewCPDockerMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/customImage",
				ValueMatcher: gomega.Equal("ghcr.io/mesosphere/kind-node:v1.2.3"),
			}},
		},
		{
			Name: "image set for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					"a-specific-image",
					clusterconfig.MetaControlPlaneConfigName,
					dockerclusterconfig.DockerVariableName,
					VariableName,
				),
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewCPDockerMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/customImage",
				ValueMatcher: gomega.Equal("a-specific-image"),
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
