// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.ClusterConfigSpec{Nutanix: &v1alpha1.NutanixSpec{}}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "machine details",
			Vals: v1alpha1.ClusterConfigSpec{
				ControlPlane: &v1alpha1.NodeConfigSpec{
					Nutanix: &v1alpha1.NutanixNodeSpec{
						MachineDetails: &v1alpha1.NutanixMachineDetails{
							BootType:       "legacy",
							VCPUSockets:    2,
							VCPUsPerSocket: 1,
							Image:          v1alpha1.NutanixResourceIdentifier{},
							Cluster:        v1alpha1.NutanixResourceIdentifier{},
							MemorySize:     "8Gi",
							SystemDiskSize: "40Gi",
							Subnets:        []v1alpha1.NutanixResourceIdentifier{},
							Project:        v1alpha1.NutanixResourceIdentifier{},
							GPUs:           []v1alpha1.NutanixGPU{},
						},
					},
				},
			},
		},
	)
}
