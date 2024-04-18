// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	requiredFields := minimumClusterConfigSpec()

	withAdditionalCategories := minimumClusterConfigSpec()
	//nolint:lll // gofumpt formats is this way
	withAdditionalCategories.ControlPlane.Nutanix.MachineDetails.AdditionalCategories = []v1alpha1.NutanixCategoryIdentifier{
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
		v1alpha1.NutanixResourceIdentifier{
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
		v1alpha1.NutanixResourceIdentifier{
			Type: "invalid-project-type",
			Name: ptr.To("fake-project"),
		},
	)

	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.ClusterConfigSpec{Nutanix: &v1alpha1.NutanixSpec{}}.VariableSchema()),
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

func minimumClusterConfigSpec() v1alpha1.ClusterConfigSpec {
	return v1alpha1.ClusterConfigSpec{
		ControlPlane: &v1alpha1.NodeConfigSpec{
			Nutanix: &v1alpha1.NutanixNodeSpec{
				MachineDetails: v1alpha1.NutanixMachineDetails{
					BootType:       v1alpha1.NutanixBootType(capxv1.NutanixBootTypeLegacy),
					VCPUSockets:    2,
					VCPUsPerSocket: 1,
					Image: v1alpha1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To("fake-image"),
					},
					Cluster: v1alpha1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To("fake-pe-cluster"),
					},
					MemorySize:     resource.MustParse("8Gi"),
					SystemDiskSize: resource.MustParse("40Gi"),
					Subnets:        []v1alpha1.NutanixResourceIdentifier{},
				},
			},
		},
	}
}
