// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneloadbalancer

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	capav1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestControlPlaneLoadBalancerPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "AWS ControlPlane LoadBalancer mutator suite")
}

var _ = Describe("Generate AWS ControlPlane LoadBalancer patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "ControlPlaneLoadbalancer scheme set to internet-facing",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.AWSLoadBalancerSpec{
						Scheme: &capav1.ELBSchemeInternetFacing,
					},
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/controlPlaneLoadBalancer",
				ValueMatcher: gomega.HaveKeyWithValue(
					"scheme", "internet-facing",
				),
			}},
		},
		{
			Name: "ControlPlaneLoadbalancer scheme set to internal",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.AWSLoadBalancerSpec{
						Scheme: &capav1.ELBSchemeInternal,
					},
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/controlPlaneLoadBalancer",
				ValueMatcher: gomega.HaveKeyWithValue(
					"scheme", "internal",
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
