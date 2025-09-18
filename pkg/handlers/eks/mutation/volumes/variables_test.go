// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package volumes

import (
	"testing"

	"k8s.io/utils/ptr"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	eksworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/workerconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.WorkerConfigVariableName,
		ptr.To(v1alpha1.EKSWorkerNodeConfig{}.VariableSchema()),
		false,
		eksworkerconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "Volumes Specification for EKS Worker",
			Vals: v1alpha1.EKSWorkerNodeConfigSpec{
				EKS: &v1alpha1.AWSWorkerNodeSpec{
					AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
						Volumes: &v1alpha1.AWSVolumes{
							Root: &v1alpha1.AWSVolume{
								DeviceName:    "/dev/sda1",
								Size:          100,
								Type:          capav1.VolumeTypeGP3,
								IOPS:          3000,
								Throughput:    125,
								Encrypted:     true,
								EncryptionKey: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
							},
							NonRoot: []v1alpha1.AWSVolume{
								{
									DeviceName: "/dev/sdf",
									Size:       200,
									Type:       capav1.VolumeTypeGP3,
									IOPS:       4000,
									Throughput: 250,
									Encrypted:  true,
								},
							},
						},
					},
				},
			},
		},
	)
}
