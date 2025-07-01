// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanefailuredomains

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
	emptyFDs := minimumClusterConfigSpec()

	validFDs := minimumClusterConfigSpec()
	validFDs.ControlPlane.Nutanix.FailureDomains = []string{"fd-1", "fd-2", "fd-3"}

	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "empty nutanix controlplane failuredomains",
			Vals: emptyFDs,
		},
		capitest.VariableTestDef{
			Name: "valid nonempty nutanix controlplane failuredomains",
			Vals: validFDs,
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
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To("fake-pe-cluster"),
					},
					MemorySize:     resource.MustParse("8Gi"),
					SystemDiskSize: resource.MustParse("40Gi"),
					Subnets:        []capxv1.NutanixResourceIdentifier{},
				},
			},
		},
	}
}
