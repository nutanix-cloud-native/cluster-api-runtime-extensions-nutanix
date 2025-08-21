// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nodetype

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	eksbootstrapv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/bootstrap/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/testutils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestNodeTypePatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "EKS NodeType mutator suite")
}

var _ = Describe("Generate EKS NodeType patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewWorkerPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "node type set explicitly to al2023",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							NodeType: string(eksbootstrapv1.NodeTypeAL2023),
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: testutils.NewEKSConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/nodeType",
				ValueMatcher: gomega.Equal("al2023"),
			}},
		},
		{
			Name: "node type set explicitly to al2023 even with AMI ID set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							NodeType: string(eksbootstrapv1.NodeTypeAL2023),
							AWSWorkerNodeSpec: v1alpha1.AWSWorkerNodeSpec{
								AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
									AMISpec: &v1alpha1.AMISpec{
										ID: "ami-1234",
									},
								},
							},
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: testutils.NewEKSConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/nodeType",
				ValueMatcher: gomega.Equal("al2023"),
			}},
		},
		{
			Name: "node type set explicitly to al2023 even with AMI lookup set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							NodeType: string(eksbootstrapv1.NodeTypeAL2023),
							AWSWorkerNodeSpec: v1alpha1.AWSWorkerNodeSpec{
								AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
									AMISpec: &v1alpha1.AMISpec{
										Lookup: &v1alpha1.AMILookup{
											Format: "capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-*",
											Org:    "123456789012",
											BaseOS: "al2023",
										},
									},
								},
							},
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: testutils.NewEKSConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/nodeType",
				ValueMatcher: gomega.Equal("al2023"),
			}},
		},
		{
			Name: "node type not set explicitly with k8s version < 1.33.0-0",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.32.0"}}`),
					},
				),
			},
			RequestItem: testutils.NewEKSConfigTemplateRequestItem(""),
		},
		{
			Name: "node type not set explicitly with k8s version >= 1.33.0-0",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: testutils.NewEKSConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/nodeType",
				ValueMatcher: gomega.Equal("al2023"),
			}},
		},
		{
			Name: "node type not set explicitly with k8s version >= 1.33.0-0 but with AMI ID set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							AWSWorkerNodeSpec: v1alpha1.AWSWorkerNodeSpec{
								AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
									AMISpec: &v1alpha1.AMISpec{
										ID: "ami-1234",
									},
								},
							},
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: testutils.NewEKSConfigTemplateRequestItem(""),
		},
		{
			Name: "node type not set explicitly with k8s version >= 1.33.0-0 but with AMI lookup set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							AWSWorkerNodeSpec: v1alpha1.AWSWorkerNodeSpec{
								AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
									AMISpec: &v1alpha1.AMISpec{
										Lookup: &v1alpha1.AMILookup{
											Format: "capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-*",
											Org:    "123456789012",
											BaseOS: "al2023",
										},
									},
								},
							},
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: testutils.NewEKSConfigTemplateRequestItem(""),
		},
		{
			Name: "node type set explicitly to al2023 for AWSMachineTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							NodeType: string(eksbootstrapv1.NodeTypeAL2023),
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/ami/eksLookupType",
				ValueMatcher: gomega.Equal("AmazonLinux2023"),
			}},
		},
		{
			Name: "node type set explicitly to al2023 even with AMI ID set for AWSMachineTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							NodeType: string(eksbootstrapv1.NodeTypeAL2023),
							AWSWorkerNodeSpec: v1alpha1.AWSWorkerNodeSpec{
								AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
									AMISpec: &v1alpha1.AMISpec{
										ID: "ami-1234",
									},
								},
							},
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/cloudInit/insecureSkipSecretsManager",
				ValueMatcher: gomega.Equal(true),
			}},
		},
		{
			Name: "node type set explicitly to al2023 even with AMI lookup set for AWSMachineTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							NodeType: string(eksbootstrapv1.NodeTypeAL2023),
							AWSWorkerNodeSpec: v1alpha1.AWSWorkerNodeSpec{
								AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
									AMISpec: &v1alpha1.AMISpec{
										Lookup: &v1alpha1.AMILookup{
											Format: "capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-*",
											Org:    "123456789012",
											BaseOS: "al2023",
										},
									},
								},
							},
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/cloudInit/insecureSkipSecretsManager",
				ValueMatcher: gomega.Equal(true),
			}},
		},
		{
			Name: "node type not set explicitly with k8s version < 1.33.0-0 for AWSMachineTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.32.0"}}`),
					},
				),
			},
			RequestItem: testutils.NewEKSConfigTemplateRequestItem(""),
		},
		{
			Name: "node type not set explicitly with k8s version >= 1.33.0-0 for AWSMachineTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/ami/eksLookupType",
				ValueMatcher: gomega.Equal("AmazonLinux2023"),
			}, {
				Operation:    "add",
				Path:         "/spec/template/spec/cloudInit/insecureSkipSecretsManager",
				ValueMatcher: gomega.Equal(true),
			}},
		},
		{
			Name: "node type not set explicitly with k8s version >= 1.33.0-0 but with AMI ID set for AWSMachineTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							AWSWorkerNodeSpec: v1alpha1.AWSWorkerNodeSpec{
								AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
									AMISpec: &v1alpha1.AMISpec{
										ID: "ami-1234",
									},
								},
							},
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
		},
		{
			Name: "node type not set explicitly with k8s version >= 1.33.0-0 but with AMI lookup set for AWSMachineTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.EKSWorkerNodeConfigSpec{
						EKS: &v1alpha1.EKSWorkerNodeSpec{
							AWSWorkerNodeSpec: v1alpha1.AWSWorkerNodeSpec{
								AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
									AMISpec: &v1alpha1.AMISpec{
										Lookup: &v1alpha1.AMILookup{
											Format: "capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-*",
											Org:    "123456789012",
											BaseOS: "al2023",
										},
									},
								},
							},
						},
					},
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker", "version": "1.33.0"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
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
