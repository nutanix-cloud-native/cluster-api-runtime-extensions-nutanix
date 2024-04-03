// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func TestGeneratePatches(
	t *testing.T,
	generatorFunc func() mutation.GeneratePatches,
	variableName string,
	variablePath ...string,
) {
	t.Helper()
	capitest.ValidateGeneratePatches(
		t,
		generatorFunc,
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
			Name: "all fields set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.NutanixPrismCentralEndpointSpec{
						Host:     "prism-central.nutanix.com",
						Port:     9441,
						Insecure: false,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
						AdditionalTrustBundle: ptr.To(corev1.LocalObjectReference{
							Name: "bundle",
						}),
					},
					variablePath...,
				),
			},
			RequestItem: request.NewNutanixClusterTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "replace",
					Path:      "/spec/template/spec/prismCentral",
					ValueMatcher: gomega.SatisfyAll(
						gomega.HaveKeyWithValue(
							"address",
							gomega.BeEquivalentTo("prism-central.nutanix.com"),
						),
						gomega.HaveKeyWithValue("port", gomega.BeEquivalentTo(9441)),
						gomega.HaveKeyWithValue("insecure", false),
						gomega.HaveKey("credentialRef"),
						gomega.HaveKey("additionalTrustBundle"),
					),
				},
			},
		},
	)
}
