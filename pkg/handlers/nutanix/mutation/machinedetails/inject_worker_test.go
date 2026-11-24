// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	. "github.com/onsi/ginkgo/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	capitestrequest "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate Nutanix Machine Details patches for Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewWorkerPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "all fields set for workers",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					variableWithAllFieldsSet,
					v1alpha1.NutanixVariableName,
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem:           request.NewWorkerNutanixMachineTemplateRequestItem(""),
			ExpectedPatchMatchers: matchersForAllFieldsSet,
		},
		{
			Name: "vmProfile set for workers clears cpu/memory/bootType/gpus",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					variableWithVMProfile,
					v1alpha1.NutanixVariableName,
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: func() runtimehooksv1.GeneratePatchesRequestItem {
				return capitestrequest.NewRequestItem(
					&capxv1.NutanixMachineTemplate{
						TypeMeta: metav1.TypeMeta{
							APIVersion: capxv1.GroupVersion.String(),
							Kind:       "NutanixMachineTemplate",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "nutanix-machine-template",
							Namespace: "nutanix-cluster",
						},
						Spec: capxv1.NutanixMachineTemplateSpec{
							Template: capxv1.NutanixMachineTemplateResource{
								Spec: capxv1.NutanixMachineSpec{
									BootType:       capxv1.NutanixBootTypeLegacy,
									VCPUSockets:    2,
									VCPUsPerSocket: 1,
									MemorySize:     resource.MustParse("8Gi"),
									SystemDiskSize: resource.MustParse("40Gi"),
									GPUs: []capxv1.NutanixGPU{
										{
											Type: "name",
											Name: new("gpu1"),
										},
									},
								},
							},
						},
					},
					&runtimehooksv1.HolderReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "MachineDeployment",
						FieldPath:  "spec.template.spec.infrastructureRef",
					},
					"",
				)
			}(),
			ExpectedPatchMatchers: matchersForVMProfileSet,
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
