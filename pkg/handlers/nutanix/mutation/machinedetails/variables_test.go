// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	capxv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	testImageName := "fake-image"
	testPEClusterName := "fake-pe-cluster"
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.ClusterConfigSpec{Nutanix: &v1alpha1.NutanixSpec{}}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "all fields set",
			Vals: v1alpha1.ClusterConfigSpec{
				ControlPlane: &v1alpha1.NodeConfigSpec{
					Nutanix: &v1alpha1.NutanixNodeSpec{
						MachineDetails: v1alpha1.NutanixMachineDetails{
							BootType:       v1alpha1.NutanixBootType(capxv1.NutanixBootTypeLegacy),
							VCPUSockets:    2,
							VCPUsPerSocket: 1,
							Image: v1alpha1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: &testImageName,
							},
							Cluster: v1alpha1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: &testPEClusterName,
							},
							MemorySize:     resource.MustParse("8Gi"),
							SystemDiskSize: resource.MustParse("40Gi"),
							Subnets:        []v1alpha1.NutanixResourceIdentifier{},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid boot type",
			Vals: v1alpha1.ClusterConfigSpec{
				ControlPlane: &v1alpha1.NodeConfigSpec{
					Nutanix: &v1alpha1.NutanixNodeSpec{
						MachineDetails: v1alpha1.NutanixMachineDetails{
							BootType:       "invalid",
							VCPUSockets:    2,
							VCPUsPerSocket: 1,
							Image: v1alpha1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: &testImageName,
							},
							Cluster: v1alpha1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: &testPEClusterName,
							},
							MemorySize:     resource.MustParse("8Gi"),
							SystemDiskSize: resource.MustParse("40Gi"),
							Subnets:        []v1alpha1.NutanixResourceIdentifier{},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "invalid image type",
			Vals: v1alpha1.ClusterConfigSpec{
				ControlPlane: &v1alpha1.NodeConfigSpec{
					Nutanix: &v1alpha1.NutanixNodeSpec{
						MachineDetails: v1alpha1.NutanixMachineDetails{
							BootType:       v1alpha1.NutanixBootType(capxv1.NutanixBootTypeLegacy),
							VCPUSockets:    2,
							VCPUsPerSocket: 1,
							Image: v1alpha1.NutanixResourceIdentifier{
								Type: "invalid",
								Name: &testImageName,
							},
							Cluster: v1alpha1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: &testPEClusterName,
							},
							MemorySize:     resource.MustParse("8Gi"),
							SystemDiskSize: resource.MustParse("40Gi"),
							Subnets:        []v1alpha1.NutanixResourceIdentifier{},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "invalid cluster type",
			Vals: v1alpha1.ClusterConfigSpec{
				ControlPlane: &v1alpha1.NodeConfigSpec{
					Nutanix: &v1alpha1.NutanixNodeSpec{
						MachineDetails: v1alpha1.NutanixMachineDetails{
							BootType:       v1alpha1.NutanixBootType(capxv1.NutanixBootTypeLegacy),
							VCPUSockets:    2,
							VCPUsPerSocket: 1,
							Image: v1alpha1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: &testImageName,
							},
							Cluster: v1alpha1.NutanixResourceIdentifier{
								Type: "invalid",
								Name: &testPEClusterName,
							},
							MemorySize:     resource.MustParse("8Gi"),
							SystemDiskSize: resource.MustParse("40Gi"),
							Subnets:        []v1alpha1.NutanixResourceIdentifier{},
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}
