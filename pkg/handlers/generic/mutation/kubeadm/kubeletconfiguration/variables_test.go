// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testDefs = []capitest.VariableTestDef{
	{
		Name: "unset",
		Vals: v1alpha1.DockerClusterConfigSpec{},
	},
	{
		Name: "set with string resource quantities",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					KubeReserved: map[string]resource.Quantity{
						"cpu":    resource.MustParse("500m"),
						"memory": resource.MustParse("256Mi"),
					},
					SystemReserved: map[string]resource.Quantity{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("128Mi"),
					},
					ContainerLogMaxSize: ptr.To(resource.MustParse("50Mi")),
				},
			},
		},
	},
	{
		Name: "set with all kubelet fields",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					MaxPods: ptr.To(int32(110)),
					KubeReserved: map[string]resource.Quantity{
						"cpu": resource.MustParse("500m"),
					},
					SystemReserved: map[string]resource.Quantity{
						"cpu": resource.MustParse("100m"),
					},
					EvictionHard: map[string]string{
						"memory.available": "100Mi",
					},
					EvictionSoft: map[string]string{
						"memory.available": "200Mi",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: 30 * time.Second},
					},
					ProtectKernelDefaults: ptr.To(true),
					TopologyManagerPolicy: ptr.To(v1alpha1.TopologyManagerPolicyNone),
					CPUManagerPolicy:      ptr.To(v1alpha1.CPUManagerPolicyStatic),
					MemoryManagerPolicy:   ptr.To(v1alpha1.MemoryManagerPolicyNone),
					PodPidsLimit:          ptr.To(int64(4096)),
					ContainerLogMaxSize:   ptr.To(resource.MustParse("10Mi")),
					ContainerLogMaxFiles:  ptr.To(int32(5)),
					MaxParallelImagePulls: ptr.To(int32(5)),
					ShutdownGracePeriod:   &metav1.Duration{Duration: 60 * time.Second},
					ShutdownGracePeriodCriticalPods: &metav1.Duration{
						Duration: 10 * time.Second,
					},
				},
			},
		},
	},
	{
		Name: "suffixless string quantity in kubeReserved",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					KubeReserved: map[string]resource.Quantity{
						"cpu": resource.MustParse("2"),
					},
				},
			},
		},
	},
	{
		Name: "suffixless string quantity in containerLogMaxSize",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					ContainerLogMaxSize: ptr.To(resource.MustParse("50")),
				},
			},
		},
	},
	{
		Name: "bare integer in kubeReserved rejected by string-only schema",
		Vals: map[string]any{
			"kubeletConfiguration": map[string]any{
				"kubeReserved": map[string]any{
					"cpu": 2,
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "bare integer in containerLogMaxSize rejected by string-only schema",
		Vals: map[string]any{
			"kubeletConfiguration": map[string]any{
				"containerLogMaxSize": 50,
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid maxPods below minimum",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					MaxPods: ptr.To(int32(10)),
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid kubeReserved key",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					KubeReserved: map[string]resource.Quantity{
						"gpu": resource.MustParse("1"),
					},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid systemReserved key",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					SystemReserved: map[string]resource.Quantity{
						"gpu": resource.MustParse("1"),
					},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "invalid evictionHard key",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					EvictionHard: map[string]string{
						"invalid.signal": "100Mi",
					},
				},
			},
		},
		ExpectError: true,
	},
	{
		Name: "imageGCHighThresholdPercent must be greater than low",
		Vals: v1alpha1.DockerClusterConfigSpec{
			KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
				KubeletConfiguration: &v1alpha1.KubeletConfiguration{
					ImageGCHighThresholdPercent: ptr.To(int32(70)),
					ImageGCLowThresholdPercent:  ptr.To(int32(80)),
				},
			},
		},
		ExpectError: true,
	},
}

func TestVariableValidation_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_AWS(t *testing.T) {
	awsTestDefs := make([]capitest.VariableTestDef, len(testDefs))
	for i, td := range testDefs {
		awsTestDefs[i] = capitest.VariableTestDef{
			Name:        td.Name,
			ExpectError: td.ExpectError,
		}
		if dockerSpec, ok := td.Vals.(v1alpha1.DockerClusterConfigSpec); ok {
			awsTestDefs[i].Vals = v1alpha1.AWSClusterConfigSpec{
				KubeadmClusterConfigSpec: dockerSpec.KubeadmClusterConfigSpec,
				ControlPlane: &v1alpha1.AWSControlPlaneSpec{
					AWS: &v1alpha1.AWSControlPlaneNodeSpec{
						InstanceType: "t3.medium",
					},
				},
			}
		} else {
			awsTestDefs[i].Vals = td.Vals
		}
	}

	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		awsTestDefs...,
	)
}

func TestVariableValidation_Nutanix(t *testing.T) {
	nutanixTestDefs := make([]capitest.VariableTestDef, len(testDefs))
	for i, td := range testDefs {
		nutanixTestDefs[i] = capitest.VariableTestDef{
			Name:        td.Name,
			ExpectError: td.ExpectError,
		}
		if dockerSpec, ok := td.Vals.(v1alpha1.DockerClusterConfigSpec); ok {
			nutanixTestDefs[i].Vals = v1alpha1.NutanixClusterConfigSpec{
				KubeadmClusterConfigSpec: dockerSpec.KubeadmClusterConfigSpec,
				ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
					Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
						MachineDetails: v1alpha1.NutanixMachineDetails{
							BootType:       capxv1.NutanixBootTypeLegacy,
							VCPUSockets:    2,
							VCPUsPerSocket: 1,
							Image: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("fake-image"),
							},
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("fake-pe-cluster"),
							},
							MemorySize:     resource.MustParse("8Gi"),
							SystemDiskSize: resource.MustParse("40Gi"),
							Subnets: []capxv1.NutanixResourceIdentifier{
								{
									Type: capxv1.NutanixIdentifierName,
									Name: ptr.To("fake-subnet"),
								},
							},
						},
					},
				},
			}
		} else {
			nutanixTestDefs[i].Vals = td.Vals
		}
	}

	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		nutanixTestDefs...,
	)
}
