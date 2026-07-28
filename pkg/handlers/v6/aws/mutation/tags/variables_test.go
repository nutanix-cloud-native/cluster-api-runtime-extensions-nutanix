// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tags

import (
	"testing"

	"k8s.io/utils/ptr"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	awsworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/workerconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "AdditionalTags at cluster level",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					AdditionalTags: capav1.Tags{
						"Environment": "production",
						"Team":        "platform",
						"CostCenter":  "12345",
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "AdditionalTags at control plane level",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ControlPlane: &v1alpha1.AWSControlPlaneSpec{
					AWS: &v1alpha1.AWSControlPlaneNodeSpec{
						AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
							AdditionalTags: capav1.Tags{
								"NodeType":    "control-plane",
								"Environment": "production",
								"Team":        "platform",
							},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "AdditionalTags at both cluster and control plane levels",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					AdditionalTags: capav1.Tags{
						"Environment": "production",
						"Team":        "platform",
						"CostCenter":  "12345",
					},
				},
				ControlPlane: &v1alpha1.AWSControlPlaneSpec{
					AWS: &v1alpha1.AWSControlPlaneNodeSpec{
						AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
							AdditionalTags: capav1.Tags{
								"NodeType":    "control-plane",
								"Environment": "production",
								"Team":        "platform",
							},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "AdditionalTags with special characters",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					AdditionalTags: capav1.Tags{
						"kubernetes.io/cluster/test-cluster": "owned",
						"Name":                               "test-cluster",
						"Environment":                        "dev",
						"Team":                               "platform",
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "Empty AdditionalTags",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					AdditionalTags: capav1.Tags{},
				},
			},
		},
	)
}

func TestWorkerVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.WorkerConfigVariableName,
		ptr.To(v1alpha1.AWSWorkerNodeConfig{}.VariableSchema()),
		false,
		awsworkerconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "AdditionalTags for workers",
			Vals: v1alpha1.AWSWorkerNodeConfigSpec{
				AWS: &v1alpha1.AWSWorkerNodeSpec{
					AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
						AdditionalTags: capav1.Tags{
							"Environment": "production",
							"Team":        "platform",
							"CostCenter":  "12345",
							"NodeType":    "worker",
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "AdditionalTags with special characters for workers",
			Vals: v1alpha1.AWSWorkerNodeConfigSpec{
				AWS: &v1alpha1.AWSWorkerNodeSpec{
					AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
						AdditionalTags: capav1.Tags{
							"kubernetes.io/cluster/test-cluster": "owned",
							"Name":                               "test-cluster-worker",
							"Environment":                        "dev",
							"NodeType":                           "worker",
							"Team":                               "platform",
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "Empty AdditionalTags for workers",
			Vals: v1alpha1.AWSWorkerNodeConfigSpec{
				AWS: &v1alpha1.AWSWorkerNodeSpec{
					AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
						AdditionalTags: capav1.Tags{},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "AdditionalTags with AWS resource naming for workers",
			Vals: v1alpha1.AWSWorkerNodeConfigSpec{
				AWS: &v1alpha1.AWSWorkerNodeSpec{
					AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
						AdditionalTags: capav1.Tags{
							"aws:autoscaling:groupName": "test-cluster-worker-asg",
							"aws:ec2:instanceType":      "m5.large",
							"Environment":               "production",
							"Team":                      "platform",
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "AdditionalTags with cost allocation for workers",
			Vals: v1alpha1.AWSWorkerNodeConfigSpec{
				AWS: &v1alpha1.AWSWorkerNodeSpec{
					AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
						AdditionalTags: capav1.Tags{
							"CostCenter":  "12345",
							"Project":     "kubernetes-cluster",
							"Owner":       "platform-team",
							"Environment": "production",
							"NodeType":    "worker",
						},
					},
				},
			},
		},
	)
}
