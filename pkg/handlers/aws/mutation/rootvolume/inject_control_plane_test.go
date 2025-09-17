// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package rootvolume

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate RootVolume patches for ControlPlane", func() {
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
			Name: "RootVolume for controlplane set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.Volume{
						DeviceName:    "/dev/sda1",
						Size:          100,
						Type:          capav1.VolumeTypeGP3,
						IOPS:          3000,
						Throughput:    125,
						Encrypted:     true,
						EncryptionKey: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
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
			Name: "RootVolume with minimal configuration",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.Volume{
						Size: 50,
						Type: capav1.VolumeTypeGP2,
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
