// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
)

func TestControlPlaneGeneratePatches(
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
			Name: "AMI set for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.AMISpec{ID: "ami-controlplane"},
					variablePath...,
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
		capitest.PatchTestDef{
			Name: "AMI lookup format set for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.AMISpec{
						Lookup: &v1alpha1.AMILookup{
							Format: "test-{{.kubernetesVersion}}-format",
							Org:    "12345",
							BaseOS: "testOS",
						},
					},
					variablePath...,
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
					ValueMatcher: gomega.Equal("12345"),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/imageLookupBaseOS",
					ValueMatcher: gomega.Equal("testOS"),
				},
			},
		},
	)
}

func TestWorkerGeneratePatches(
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
			Name: "AMI set for workers",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.AMISpec{ID: "ami-controlplane"},
					variablePath...,
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
		capitest.PatchTestDef{
			Name: "AMI lookup format set for worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.AMISpec{
						Lookup: &v1alpha1.AMILookup{
							Format: "test-{{.kubernetesVersion}}-format",
							Org:    "12345",
							BaseOS: "testOS",
						},
					},

					variablePath...,
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
		},
	)
}
