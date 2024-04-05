// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	capav1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestCalicoPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Calico CNI ingress mutator suite")
}

var _ = Describe("Generate AWS Calico CNI ingress patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "provider set with AWSClusterTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.CNI{
						Provider: v1alpha1.CNIProviderCalico,
					},
					"addons",
					v1alpha1.CNIVariableName,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/network/cni",
				ValueMatcher: gomega.HaveKeyWithValue(
					"cniIngressRules",
					gomega.ConsistOf(
						gomega.SatisfyAll(
							gomega.HaveKeyWithValue("description", "typha (calico)"),
							gomega.HaveKeyWithValue(
								"protocol",
								gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
							),
							gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(5473)),
							gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(5473)),
						),
						gomega.SatisfyAll(
							gomega.HaveKeyWithValue("description", "bgp (calico)"),
							gomega.HaveKeyWithValue(
								"protocol",
								gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
							),
							gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(179)),
							gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(179)),
						),
						gomega.SatisfyAll(
							gomega.HaveKeyWithValue("description", "IP-in-IP (calico)"),
							gomega.HaveKeyWithValue(
								"protocol",
								gomega.BeEquivalentTo(capav1.SecurityGroupProtocolIPinIP),
							),
							gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(-1)),
							gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(65535)),
						),
						gomega.SatisfyAll(
							gomega.HaveKeyWithValue("description", "node metrics (calico)"),
							gomega.HaveKeyWithValue(
								"protocol",
								gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
							),
							gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(9091)),
							gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(9091)),
						),
						gomega.SatisfyAll(
							gomega.HaveKeyWithValue("description", "typha metrics (calico)"),
							gomega.HaveKeyWithValue(
								"protocol",
								gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
							),
							gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(9093)),
							gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(9093)),
						),
					),
				),
			}},
		},
		{
			Name: "provider set with AWSClusterTemplate pre-existing rules",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.CNI{
						Provider: v1alpha1.CNIProviderCalico,
					},
					"addons",
					v1alpha1.CNIVariableName,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem(
				"1234",
				capav1.AWSClusterTemplateSpec{
					Template: capav1.AWSClusterTemplateResource{
						Spec: capav1.AWSClusterSpec{
							NetworkSpec: capav1.NetworkSpec{
								CNI: &capav1.CNISpec{
									CNIIngressRules: []capav1.CNIIngressRule{{
										Description: "test",
										Protocol:    capav1.SecurityGroupProtocolAll,
										FromPort:    1234,
										ToPort:      12345,
									}},
								},
							},
						},
					},
				},
			),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/1",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "typha (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(5473)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(5473)),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/2",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "bgp (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(179)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(179)),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/3",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "IP-in-IP (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolIPinIP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(-1)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(65535)),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/4",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "node metrics (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(9091)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(9091)),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/5",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "typha metrics (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(9093)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(9093)),
				),
			}},
		},
		{
			Name: "provider set with AWSClusterTemplate conflicting pre-existing rules",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.CNI{
						Provider: v1alpha1.CNIProviderCalico,
					},
					"addons",
					v1alpha1.CNIVariableName,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem(
				"1234",
				capav1.AWSClusterTemplateSpec{
					Template: capav1.AWSClusterTemplateResource{
						Spec: capav1.AWSClusterSpec{
							NetworkSpec: capav1.NetworkSpec{
								CNI: &capav1.CNISpec{
									CNIIngressRules: []capav1.CNIIngressRule{{
										Description: "typha (calico)",
										Protocol:    capav1.SecurityGroupProtocolTCP,
										FromPort:    5473,
										ToPort:      5473,
									}},
								},
							},
						},
					},
				},
			),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/1",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "bgp (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(179)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(179)),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/2",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "IP-in-IP (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolIPinIP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(-1)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(65535)),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/3",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "node metrics (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(9091)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(9091)),
				),
			}, {
				Operation: "add",
				Path:      "/spec/template/spec/network/cni/cniIngressRules/4",
				ValueMatcher: gomega.SatisfyAll(
					gomega.HaveKeyWithValue("description", "typha metrics (calico)"),
					gomega.HaveKeyWithValue(
						"protocol",
						gomega.BeEquivalentTo(capav1.SecurityGroupProtocolTCP),
					),
					gomega.HaveKeyWithValue("fromPort", gomega.BeEquivalentTo(9093)),
					gomega.HaveKeyWithValue("toPort", gomega.BeEquivalentTo(9093)),
				),
			}},
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
