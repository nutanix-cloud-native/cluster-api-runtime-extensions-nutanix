// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package autorenewcerts

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func TestAutoRenewCertsPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Auto renew certs mutator suite")
}

type testObj struct {
	patchTest capitest.PatchTestDef
}

var _ = Describe("Generate auto renew patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", nil, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []testObj{
		{
			patchTest: capitest.PatchTestDef{
				Name: "auto cert renewal set with AWS",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.AWSClusterConfigSpec{
							ControlPlane: &v1alpha1.AWSControlPlaneSpec{
								GenericControlPlaneSpec: v1alpha1.GenericControlPlaneSpec{
									AutoRenewCertificates: &v1alpha1.AutoRenewCertificatesSpec{
										DaysBeforeExpiry: 10,
									},
								},
							},
						},
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/rolloutBefore",
					ValueMatcher: gomega.HaveKeyWithValue(
						"certificatesExpiryDays",
						gomega.BeEquivalentTo(10),
					),
				}},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "auto cert renewal set with Docker",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.DockerClusterConfigSpec{
							ControlPlane: &v1alpha1.DockerControlPlaneSpec{
								GenericControlPlaneSpec: v1alpha1.GenericControlPlaneSpec{
									AutoRenewCertificates: &v1alpha1.AutoRenewCertificatesSpec{
										DaysBeforeExpiry: 10,
									},
								},
							},
						},
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/rolloutBefore",
					ValueMatcher: gomega.HaveKeyWithValue(
						"certificatesExpiryDays",
						gomega.BeEquivalentTo(10),
					),
				}},
			},
		},
		{
			patchTest: capitest.PatchTestDef{
				Name: "auto cert renewal set with Nutanix",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						v1alpha1.NutanixClusterConfigSpec{
							ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
								GenericControlPlaneSpec: v1alpha1.GenericControlPlaneSpec{
									AutoRenewCertificates: &v1alpha1.AutoRenewCertificatesSpec{
										DaysBeforeExpiry: 10,
									},
								},
							},
						},
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
					Operation: "add",
					Path:      "/spec/template/spec/rolloutBefore",
					ValueMatcher: gomega.HaveKeyWithValue(
						"certificatesExpiryDays",
						gomega.BeEquivalentTo(10),
					),
				}},
			},
		},
	}

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.patchTest.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt.patchTest)
		})
	}
})
