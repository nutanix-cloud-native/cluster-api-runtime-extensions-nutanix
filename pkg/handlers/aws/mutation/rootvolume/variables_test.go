// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package rootvolume

import (
	"testing"

	"k8s.io/utils/ptr"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "Root Volume Specification",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ControlPlane: &v1alpha1.AWSControlPlaneSpec{
					AWS: &v1alpha1.AWSControlPlaneNodeSpec{
						AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
							RootVolume: &v1alpha1.Volume{
								DeviceName:    "/dev/sda1",
								Size:          100,
								Type:          capav1.VolumeTypeGP3,
								IOPS:          3000,
								Throughput:    125,
								Encrypted:     true,
								EncryptionKey: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
							},
						},
					},
				},
			},
		},
	)
}
