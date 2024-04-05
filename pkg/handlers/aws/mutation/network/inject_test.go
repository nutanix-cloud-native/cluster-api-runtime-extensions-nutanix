// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestNetworkPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Network mutator suite")
}

var _ = Describe("Generate AWS Network patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "VPC ID set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.AWSNetwork{
						VPC: &v1alpha1.VPC{
							ID: "vpc-1234",
						},
					},
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/network/vpc/id",
				ValueMatcher: gomega.Equal("vpc-1234"),
			}},
		},
		{
			Name: "Subnet IDs set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.AWSNetwork{
						Subnets: v1alpha1.Subnets{
							{ID: "subnet-1"},
							{ID: "subnet-2"},
							{ID: "subnet-3"},
						},
					},
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/network/subnets",
				ValueMatcher: gomega.HaveLen(3),
			}},
		},
		{
			Name: "both VPC ID and Subnet IDs set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.AWSNetwork{
						VPC: &v1alpha1.VPC{
							ID: "vpc-1234",
						},
						Subnets: v1alpha1.Subnets{
							{ID: "subnet-1"},
							{ID: "subnet-2"},
							{ID: "subnet-3"},
						},
					},
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/network/vpc/id",
				ValueMatcher: gomega.Equal("vpc-1234"),
			}, {
				Operation:    "add",
				Path:         "/spec/template/spec/network/subnets",
				ValueMatcher: gomega.HaveLen(3),
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
