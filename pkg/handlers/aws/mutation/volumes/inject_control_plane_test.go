// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package volumes

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	capav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate Volumes patches for ControlPlane", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			helpers.TestEnv.Client,
			NewControlPlanePatch(),
		).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "Root volume for controlplane set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AWSVolumes{
						Root: &v1alpha1.AWSVolume{
							DeviceName:    "/dev/sda1",
							Size:          100,
							Type:          capav1.VolumeTypeGP3,
							IOPS:          3000,
							Throughput:    125,
							Encrypted:     true,
							EncryptionKey: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
						},
					},
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewCPAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/rootVolume",
					ValueMatcher: gomega.And(
						gomega.HaveKeyWithValue("deviceName", "/dev/sda1"),
						gomega.HaveKeyWithValue("size", float64(100)),
						gomega.HaveKeyWithValue("type", "gp3"),
						gomega.HaveKeyWithValue("iops", float64(3000)),
						gomega.HaveKeyWithValue("throughput", float64(125)),
						gomega.HaveKeyWithValue("encrypted", true),
						gomega.HaveKeyWithValue(
							"encryptionKey",
							"arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
						),
					),
				},
			},
		},
		{
			Name: "Non-root volumes for controlplane set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AWSVolumes{
						NonRoot: []v1alpha1.AWSVolume{
							{
								DeviceName:    "/dev/sdf",
								Size:          200,
								Type:          capav1.VolumeTypeGP3,
								IOPS:          4000,
								Throughput:    250,
								Encrypted:     true,
								EncryptionKey: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
							},
							{
								DeviceName: "/dev/sdg",
								Size:       500,
								Type:       capav1.VolumeTypeGP2,
								Encrypted:  false,
							},
						},
					},
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewCPAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/nonRootVolumes",
					ValueMatcher: gomega.And(
						gomega.HaveLen(2),
						gomega.ContainElement(gomega.And(
							gomega.HaveKeyWithValue("deviceName", "/dev/sdf"),
							gomega.HaveKeyWithValue("size", float64(200)),
							gomega.HaveKeyWithValue("type", "gp3"),
							gomega.HaveKeyWithValue("iops", float64(4000)),
							gomega.HaveKeyWithValue("throughput", float64(250)),
							gomega.HaveKeyWithValue("encrypted", true),
						)),
						gomega.ContainElement(gomega.And(
							gomega.HaveKeyWithValue("deviceName", "/dev/sdg"),
							gomega.HaveKeyWithValue("size", float64(500)),
							gomega.HaveKeyWithValue("type", "gp2"),
						)),
					),
				},
			},
		},
		{
			Name: "Both root and non-root volumes for controlplane set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AWSVolumes{
						Root: &v1alpha1.AWSVolume{
							Size: 50,
							Type: capav1.VolumeTypeGP2,
						},
						NonRoot: []v1alpha1.AWSVolume{
							{
								DeviceName: "/dev/sdf",
								Size:       100,
								Type:       capav1.VolumeTypeGP3,
							},
						},
					},
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.AWSVariableName,
					VariableName,
				),
			},
			RequestItem: request.NewCPAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/rootVolume",
					ValueMatcher: gomega.And(
						gomega.HaveKeyWithValue("size", float64(50)),
						gomega.HaveKeyWithValue("type", "gp2"),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/nonRootVolumes",
					ValueMatcher: gomega.And(
						gomega.HaveLen(1),
						gomega.ContainElement(gomega.And(
							gomega.HaveKeyWithValue("deviceName", "/dev/sdf"),
							gomega.HaveKeyWithValue("size", float64(100)),
							gomega.HaveKeyWithValue("type", "gp3"),
						)),
					),
				},
			},
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
