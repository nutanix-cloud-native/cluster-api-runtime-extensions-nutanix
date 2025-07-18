// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	requiredFields := minimumClusterConfigSpec()

	withAdditionalCategories := minimumClusterConfigSpec()

	withAdditionalCategories.ControlPlane.Nutanix.MachineDetails.AdditionalCategories = []capxv1.NutanixCategoryIdentifier{
		{
			Key:   "fake-key",
			Value: "fake-value1",
		},
		{
			Key:   "fake-key",
			Value: "fake-value2",
		},
	}

	withProject := minimumClusterConfigSpec()
	withProject.ControlPlane.Nutanix.MachineDetails.Project = ptr.To(
		capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("fake-project"),
		},
	)

	invalidBootType := minimumClusterConfigSpec()
	invalidBootType.ControlPlane.Nutanix.MachineDetails.BootType = "invalid-boot-type"

	invalidImageType := minimumClusterConfigSpec()
	invalidImageType.ControlPlane.Nutanix.MachineDetails.Image.Type = "invalid-image-type"

	invalidClusterType := minimumClusterConfigSpec()
	invalidClusterType.ControlPlane.Nutanix.MachineDetails.Cluster.Type = "invalid-cluster-type"

	invalidProjectType := minimumClusterConfigSpec()
	invalidProjectType.ControlPlane.Nutanix.MachineDetails.Project = ptr.To(
		capxv1.NutanixResourceIdentifier{
			Type: "invalid-project-type",
			Name: ptr.To("fake-project"),
		},
	)

	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "required fields set",
			Vals: requiredFields,
		},
		capitest.VariableTestDef{
			Name: "additional categories set",
			Vals: withAdditionalCategories,
		},
		capitest.VariableTestDef{
			Name: "project set",
			Vals: withProject,
		},
		capitest.VariableTestDef{
			Name:        "invalid boot type",
			Vals:        invalidBootType,
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name:        "invalid image type",
			Vals:        invalidImageType,
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name:        "invalid cluster type",
			Vals:        invalidClusterType,
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name:        "invalid project type",
			Vals:        invalidProjectType,
			ExpectError: true,
		},
	)
}

func minimumClusterConfigSpec() v1alpha1.NutanixClusterConfigSpec {
	return v1alpha1.NutanixClusterConfigSpec{
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
}
