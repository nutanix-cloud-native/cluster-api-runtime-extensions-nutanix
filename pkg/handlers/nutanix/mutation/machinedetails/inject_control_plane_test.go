// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var (
	variableWithAllFieldsSet = v1alpha1.NutanixMachineDetails{
		BootType:       capxv1.NutanixBootTypeLegacy,
		VCPUSockets:    2,
		VCPUsPerSocket: 1,
		Image: capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("fake-image"),
		},
		Cluster: capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("fake-pe-cluster"),
		},
		MemorySize:     resource.MustParse("8Gi"),
		SystemDiskSize: resource.MustParse("40Gi"),
		Subnets: []capxv1.NutanixResourceIdentifier{
			{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("fake-subnet"),
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
		Project: ptr.To(capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("fake-project"),
		}),
		GPUs: []capxv1.NutanixGPU{
			{
				Type: "name",
				Name: ptr.To("gpu1"),
			},
			{
				Type:     "deviceID",
				DeviceID: ptr.To(int64(1)),
			},
		},
	}

	matchersForAllFieldsSet = []capitest.JSONPatchMatcher{
		{
			Operation: "add",
			Path:      "/spec/template/spec/gpus",
			ValueMatcher: gomega.ContainElements(
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue("type", "name"),
					gomega.HaveKeyWithValue("name", "gpu1"),
				),
				gomega.SatisfyAll(
					gomega.HaveKeyWithValue("type", "deviceID"),
					gomega.HaveKeyWithValue("deviceID", gomega.BeNumerically("==", 1)),
				),
			),
		},
		{
			Operation:    "add",
			Path:         "/spec/template/spec/bootType",
			ValueMatcher: gomega.BeEquivalentTo(capxv1.NutanixBootTypeLegacy),
		},
		{
			Operation:    "add",
			Path:         "/spec/template/spec/image/name",
			ValueMatcher: gomega.BeEquivalentTo("fake-image"),
		},
		{
			Operation:    "replace",
			Path:         "/spec/template/spec/image/type",
			ValueMatcher: gomega.BeEquivalentTo(capxv1.NutanixIdentifierName),
		},
		{
			Operation:    "add",
			Path:         "/spec/template/spec/cluster/name",
			ValueMatcher: gomega.BeEquivalentTo("fake-pe-cluster"),
		},
		{
			Operation:    "replace",
			Path:         "/spec/template/spec/cluster/type",
			ValueMatcher: gomega.BeEquivalentTo(capxv1.NutanixIdentifierName),
		},
		{
			Operation:    "replace",
			Path:         "/spec/template/spec/vcpuSockets",
			ValueMatcher: gomega.BeEquivalentTo(2),
		},
		{
			Operation:    "replace",
			Path:         "/spec/template/spec/vcpusPerSocket",
			ValueMatcher: gomega.BeEquivalentTo(1),
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
			Operation:    "replace",
			Path:         "/spec/template/spec/subnet",
			ValueMatcher: gomega.HaveLen(1),
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
	}

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
		})
	}
})
