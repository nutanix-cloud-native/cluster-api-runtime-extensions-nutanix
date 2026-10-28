// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	capitestrequest "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var (
	matchGPUList = func() gomega.OmegaMatcher {
		return gomega.SatisfyAll(
			// Must be a list with 3 entries.
			gomega.HaveLen(3),
			// Must contain the name-based GPU.
			gomega.ContainElement(gomega.SatisfyAll(
				gomega.HaveKeyWithValue("type", "name"),
				gomega.HaveKeyWithValue("name", "gpu1"),
			)),
			// Must contain the deviceID-based GPU. deviceID may decode as float64/int/json.Number, so use BeNumerically.
			gomega.ContainElement(gomega.SatisfyAll(
				gomega.HaveKeyWithValue("type", "deviceID"),
				gomega.HaveKey("deviceID"),
				gomega.WithTransform(
					func(m map[string]any) any { return m["deviceID"] },
					gomega.BeNumerically("==", 1),
				),
			)),
			// Must contain the profile-based GPU (AHV GPU profile, PC 7.6+).
			gomega.ContainElement(gomega.SatisfyAll(
				gomega.HaveKeyWithValue("type", "profile"),
				gomega.HaveKeyWithValue("profile", gomega.SatisfyAll(
					gomega.HaveKeyWithValue("type", "name"),
					gomega.HaveKeyWithValue("name", "gpu-profile-1"),
				)),
			)),
		)
	}

	variableWithAllFieldsSet = v1alpha1.NutanixMachineDetails{
		BootType:       capxv1.NutanixBootTypeLegacy,
		VCPUSockets:    2,
		VCPUsPerSocket: 1,
		Image: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-image"),
		},
		Cluster: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-pe-cluster"),
		},
		MemorySize:     resource.MustParse("8Gi"),
		SystemDiskSize: resource.MustParse("40Gi"),
		Subnets: []capxv1.NutanixResourceIdentifier{
			{
				Type: capxv1.NutanixIdentifierName,
				Name: new("fake-subnet"),
			},
		},
		AdditionalCategories: []capxv1.NutanixCategoryIdentifier{
			{
				Key:   "fake-key",
				Value: "fake-value",
			},
			{
				Key:   "fake-key2",
				Value: "fake-value2",
			},
		},
		Project: new(capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-project"),
		}),
		GPUs: []capxv1.NutanixGPU{
			{
				Type: "name",
				Name: new("gpu1"),
			},
			{
				Type:     "deviceID",
				DeviceID: new(int64(1)),
			},
			{
				Type: "profile",
				Profile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("gpu-profile-1"),
				},
			},
		},
	}

	variableWithVMProfile = v1alpha1.NutanixMachineDetails{
		VMProfile: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-vm-profile"),
		},
		Image: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-image"),
		},
		Cluster: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-pe-cluster"),
		},
		SystemDiskSize: resource.MustParse("40Gi"),
	}

	variableWithImageTemplating = v1alpha1.NutanixMachineDetails{
		BootType:       capxv1.NutanixBootTypeLegacy,
		VCPUSockets:    2,
		VCPUsPerSocket: 1,
		ImageLookup: &capxv1.NutanixImageLookup{
			BaseOS: "rockylinux-9",
		},
		Cluster: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-pe-cluster"),
		},
		MemorySize:     resource.MustParse("8Gi"),
		SystemDiskSize: resource.MustParse("40Gi"),
		Subnets: []capxv1.NutanixResourceIdentifier{
			{
				Type: capxv1.NutanixIdentifierName,
				Name: new("fake-subnet"),
			},
		},
	}
	matchersForAllImageTemplating = []capitest.JSONPatchMatcher{
		{
			Operation:    "add",
			Path:         "/spec/template/spec/bootType",
			ValueMatcher: gomega.BeEquivalentTo(capxv1.NutanixBootTypeLegacy),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/cluster",
			ValueMatcher: gomega.SatisfyAll(
				gomega.HaveKeyWithValue("type", gomega.BeEquivalentTo(capxv1.NutanixIdentifierName)),
				gomega.HaveKeyWithValue("name", gomega.BeEquivalentTo("fake-pe-cluster")),
			),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/vcpuSockets",
			// JSON numbers decode as float64; compare numerically for correctness.
			ValueMatcher: gomega.BeNumerically("==", 2),
		},
		{
			Operation:    "add",
			Path:         "/spec/template/spec/vcpusPerSocket",
			ValueMatcher: gomega.BeNumerically("==", 1),
		},
		{
			Operation:    "replace",
			Path:         "/spec/template/spec/memorySize",
			ValueMatcher: gomega.BeEquivalentTo("8Gi"),
		},
		{
			Operation:    "replace",
			Path:         "/spec/template/spec/systemDiskSize",
			ValueMatcher: gomega.BeEquivalentTo("40Gi"),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/subnet",
			ValueMatcher: gomega.ContainElement(
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue("type", gomega.BeEquivalentTo(capxv1.NutanixIdentifierName)),
					gomega.HaveKeyWithValue("name", gomega.BeEquivalentTo("fake-subnet")),
				),
			),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/imageLookup",
			ValueMatcher: gomega.SatisfyAll(
				gomega.HaveKeyWithValue("baseOS", "rockylinux-9"),
			),
		},
	}

	matchersForAllFieldsSet = []capitest.JSONPatchMatcher{
		{
			Operation:    "add",
			Path:         "/spec/template/spec/gpus",
			ValueMatcher: matchGPUList(),
		},
		{
			Operation:    "add",
			Path:         "/spec/template/spec/bootType",
			ValueMatcher: gomega.BeEquivalentTo(capxv1.NutanixBootTypeLegacy),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/cluster",
			ValueMatcher: gomega.SatisfyAll(
				gomega.HaveKeyWithValue("type", gomega.BeEquivalentTo(capxv1.NutanixIdentifierName)),
				gomega.HaveKeyWithValue("name", gomega.BeEquivalentTo("fake-pe-cluster")),
			),
		},
		{
			Operation:    "add",
			Path:         "/spec/template/spec/vcpuSockets",
			ValueMatcher: gomega.BeNumerically("==", 2),
		},
		{
			Operation:    "add",
			Path:         "/spec/template/spec/vcpusPerSocket",
			ValueMatcher: gomega.BeNumerically("==", 1),
		},
		{
			Operation:    "replace",
			Path:         "/spec/template/spec/memorySize",
			ValueMatcher: gomega.BeEquivalentTo("8Gi"),
		},
		{
			Operation:    "replace",
			Path:         "/spec/template/spec/systemDiskSize",
			ValueMatcher: gomega.BeEquivalentTo("40Gi"),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/subnet",
			ValueMatcher: gomega.ContainElement(
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue("type", gomega.BeEquivalentTo(capxv1.NutanixIdentifierName)),
					gomega.HaveKeyWithValue("name", gomega.BeEquivalentTo("fake-subnet")),
				),
			),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/image",
			ValueMatcher: gomega.SatisfyAll(
				gomega.HaveKeyWithValue("type", "name"),
				gomega.HaveKeyWithValue("name", "fake-image"),
			),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/additionalCategories",
			ValueMatcher: gomega.ContainElements(
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue("key", "fake-key"),
					gomega.HaveKeyWithValue("value", "fake-value"),
				),
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue("key", "fake-key2"),
					gomega.HaveKeyWithValue("value", "fake-value2"),
				),
			),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/project",
			ValueMatcher: gomega.SatisfyAll(
				gomega.HaveKeyWithValue("type", "name"),
				gomega.HaveKeyWithValue("name", "fake-project"),
			),
		},
	}

	matchersForVMProfileSet = []capitest.JSONPatchMatcher{
		{
			Operation: "add",
			Path:      "/spec/template/spec/vmProfile",
			ValueMatcher: gomega.SatisfyAll(
				gomega.HaveKeyWithValue("type", "name"),
				gomega.HaveKeyWithValue("name", "fake-vm-profile"),
			),
		},
		{
			Operation:    "remove",
			Path:         "/spec/template/spec/vcpuSockets",
			ValueMatcher: gomega.BeNil(),
		},
		{
			Operation:    "remove",
			Path:         "/spec/template/spec/vcpusPerSocket",
			ValueMatcher: gomega.BeNil(),
		},
		{
			Operation:    "remove",
			Path:         "/spec/template/spec/memorySize",
			ValueMatcher: gomega.BeNil(),
		},
		{
			Operation:    "remove",
			Path:         "/spec/template/spec/bootType",
			ValueMatcher: gomega.BeNil(),
		},
		{
			Operation:    "remove",
			Path:         "/spec/template/spec/gpus",
			ValueMatcher: gomega.BeNil(),
		},
	}
)

var _ = Describe("Generate Nutanix Machine Details patches for ControlPlane", func() {
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
			Name: "all fields set for control-plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					variableWithAllFieldsSet,
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.NutanixVariableName,
					VariableName,
				),
			},
			RequestItem:           request.NewCPNutanixMachineTemplateRequestItem(""),
			ExpectedPatchMatchers: matchersForAllFieldsSet,
		},
		{
			Name: "image templating set for control-plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					variableWithImageTemplating,
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.NutanixVariableName,
					VariableName,
				),
			},
			RequestItem:           request.NewCPNutanixMachineTemplateRequestItem(""),
			ExpectedPatchMatchers: matchersForAllImageTemplating,
		},
		{
			Name: "vmProfile set for control-plane clears cpu/memory/bootType/gpus",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					variableWithVMProfile,
					v1alpha1.ControlPlaneConfigVariableName,
					v1alpha1.NutanixVariableName,
					VariableName,
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
						APIVersion: controlplanev1.GroupVersion.String(),
						Kind:       "KubeadmControlPlane",
						FieldPath:  "spec.machineTemplate.infrastructureRef",
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
