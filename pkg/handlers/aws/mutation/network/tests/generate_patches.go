// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
)

func TestGeneratePatches(
	t *testing.T,
	generatorFunc func() mutation.GeneratePatches,
	variableName string,
	variablePath ...string,
) {
	t.Helper()

	capitest.ValidateGeneratePatches(
		t,
		generatorFunc,
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
			Name: "VPC ID set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.AWSNetwork{
						VPC: &v1alpha1.VPC{
							ID: "vpc-1234",
						},
					},
					variablePath...,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/network/vpc/id",
				ValueMatcher: gomega.Equal("vpc-1234"),
			}},
		},
		capitest.PatchTestDef{
			Name: "Subnet IDs set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.AWSNetwork{
						Subnets: v1alpha1.Subnets{
							{ID: "subnet-1"},
							{ID: "subnet-2"},
							{ID: "subnet-3"},
						},
					},
					variablePath...,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/network/subnets",
				ValueMatcher: gomega.HaveLen(3),
			}},
		},
		capitest.PatchTestDef{
			Name: "both VPC ID and Subnet IDs set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
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
					variablePath...,
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
	)
}
