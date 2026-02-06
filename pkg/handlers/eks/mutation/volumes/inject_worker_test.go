// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package volumes

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate Volumes patches for EKS Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			helpers.TestEnv.Client,
			NewWorkerPatch(),
		).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "Root volume for EKS worker set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.AWSVolumes{
						Root: &v1alpha1.AWSVolume{
							DeviceName:    "/dev/sda1",
							Size:          200,
							Type:          capav1.VolumeTypeGP3,
							IOPS:          4000,
							Throughput:    250,
							Encrypted:     true,
							EncryptionKey: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012",
						},
					},
					v1alpha1.EKSVariableName,
					"volumes",
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/rootVolume",
					ValueMatcher: gomega.And(
						gomega.HaveKeyWithValue("deviceName", "/dev/sda1"),
						gomega.HaveKeyWithValue("size", float64(200)),
						gomega.HaveKeyWithValue("type", "gp3"),
						gomega.HaveKeyWithValue("iops", float64(4000)),
						gomega.HaveKeyWithValue("throughput", float64(250)),
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
			Name: "Non-root volumes for EKS worker set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.AWSVolumes{
						NonRoot: []v1alpha1.AWSVolume{
							{
								DeviceName: "/dev/sdf",
								Size:       100,
								Type:       capav1.VolumeTypeGP3,
								IOPS:       3000,
								Throughput: 125,
								Encrypted:  true,
							},
							{
								DeviceName: "/dev/sdg",
								Size:       200,
								Type:       capav1.VolumeTypeGP2,
								Encrypted:  false,
							},
						},
					},
					v1alpha1.EKSVariableName,
					"volumes",
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/nonRootVolumes",
					ValueMatcher: gomega.And(
						gomega.HaveLen(2),
						gomega.ContainElement(gomega.And(
							gomega.HaveKeyWithValue("deviceName", "/dev/sdf"),
							gomega.HaveKeyWithValue("size", float64(100)),
							gomega.HaveKeyWithValue("type", "gp3"),
							gomega.HaveKeyWithValue("iops", float64(3000)),
							gomega.HaveKeyWithValue("throughput", float64(125)),
							gomega.HaveKeyWithValue("encrypted", true),
						)),
						gomega.ContainElement(gomega.And(
							gomega.HaveKeyWithValue("deviceName", "/dev/sdg"),
							gomega.HaveKeyWithValue("size", float64(200)),
							gomega.HaveKeyWithValue("type", "gp2"),
						)),
					),
				},
			},
		},
		{
			Name: "Both root and non-root volumes for EKS worker set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					v1alpha1.AWSVolumes{
						Root: &v1alpha1.AWSVolume{
							Size: 80,
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
					v1alpha1.EKSVariableName,
					"volumes",
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/rootVolume",
					ValueMatcher: gomega.And(
						gomega.HaveKeyWithValue("size", float64(80)),
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
