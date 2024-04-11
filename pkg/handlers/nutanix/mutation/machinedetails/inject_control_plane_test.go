// Copyright 2023 D2iQ, Inc. All rights reserved.
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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var (
	variableWithAllFieldsSet = v1alpha1.NutanixMachineDetails{
		BootType:       v1alpha1.NutanixBootType(capxv1.NutanixBootTypeLegacy),
		VCPUSockets:    2,
		VCPUsPerSocket: 1,
		Image: v1alpha1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("fake-image"),
		},
		Cluster: v1alpha1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("fake-pe-cluster"),
		},
		MemorySize:     resource.MustParse("8Gi"),
		SystemDiskSize: resource.MustParse("40Gi"),
		Subnets: []v1alpha1.NutanixResourceIdentifier{
			{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("fake-subnet"),
			},
		},
	}

	matchersForAllFieldsSet = []capitest.JSONPatchMatcher{
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
	}
)

var _ = Describe("Generate Nutanix Machine Details patches for ControlPlane", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewControlPlanePatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "all fields set for control-plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					variableWithAllFieldsSet,
					clusterconfig.MetaControlPlaneConfigName,
					nutanixclusterconfig.NutanixVariableName,
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
