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

//nolint:lll // just a long string
const testCertBundle = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUVjekNDQTF1Z0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRUUZBRC4uQWtHQTFVRUJoTUNSMEl4CkV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGREFTQmdOVkJBb1RDMC4uMEVnVEhSa01UY3dOUVlEClZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbWx0WVhKNUlFTmxjbi4uWFJwYjI0Z1FYVjBhRzl5CmFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRXgwWkRBZUZ3MHdNRC4uVFV3TVRaYUZ3MHdNVEF5Ck1EUXhPVFV3TVRaYU1JR0hNUXN3Q1FZRFZRUUdFd0pIUWpFVE1CRUdBMS4uMjl0WlMxVGRHRjBaVEVVCk1CSUdBMVVFQ2hNTFFtVnpkQ0JEUVNCTWRHUXhOekExQmdOVkJBc1RMay4uREVnVUhWaWJHbGpJRkJ5CmFXMWhjbmtnUTJWeWRHbG1hV05oZEdsdmJpQkJkWFJvYjNKcGRIa3hGRC4uQU1UQzBKbGMzUWdRMEVnClRIUmtNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZy4uVHoybXI3U1ppQU1mUXl1CnZCak05T2lKalJhelhCWjFCalA1Q0UvV20vUnI1MDBQUksrTGg5eDVlSi4uL0FOQkUwc1RLMFpzREdNCmFrMm0xZzdvcnVJM2RZM1ZIcUl4RlR6MFRhMWQrTkFqd25MZTRuT2I3Ly4uazA1U2hoQnJKR0JLS3hiCjhuMTA0by81cDhIQXNaUGR6YkZNSXlOakp6Qk0ybzV5NUExM3dpTGl0RS4uZnlZa1F6YXhDdzBBd3psCmtWSGlJeUN1YUY0d2o1NzFwU3prdjZzdis0SURNYlQvWHBDbzhMNndUYS4uc2grZXRMRDZGdFRqWWJiCnJ2WjhSUU0xdGxLZG9NSGcycXhyYUFWKytITkJZbU5XczBkdUVkalViSi4uWEk5VHRuUzRvMUNrajdQCk9mbGppUUlEQVFBQm80SG5NSUhrTUIwR0ExVWREZ1FXQkJROHVyTUNSTC4uNUFrSXA5TkpISnc1VENCCnRBWURWUjBqQklHc01JR3BnQlE4dXJNQ1JMWVlNSFVLVTVBa0lwOU5KSC4uYVNCaWpDQmh6RUxNQWtHCkExVUVCaE1DUjBJeEV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGRC4uQW9UQzBKbGMzUWdRMEVnClRIUmtNVGN3TlFZRFZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbS4uRU5sY25ScFptbGpZWFJwCmIyNGdRWFYwYUc5eWFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRS4uREFNQmdOVkhSTUVCVEFECkFRSC9NQTBHQ1NxR1NJYjNEUUVCQkFVQUE0SUJBUUMxdVlCY3NTbmN3QS4uRENzUWVyNzcyQzJ1Y3BYCnhRVUUvQzBwV1dtNmdEa3dkNUQwRFNNREpScVYvd2VvWjR3QzZCNzNmNS4uYkxoR1lIYVhKZVNENktyClhjb093TGRTYUdtSllzbExLWkIzWklERXAwd1lUR2hndGViNkpGaVR0bi4uc2YyeGRyWWZQQ2lJQjdnCkJNQVY3R3pkYzRWc3BTNmxqckFoYmlpYXdkQmlRbFFtc0JlRno5SmtGNC4uYjNsOEJvR04rcU1hNTZZCkl0OHVuYTJnWTRsMk8vL29uODhyNUlXSmxtMUwwb0E4ZTRmUjJ5ckJIWC4uYWRzR2VGS2t5TnJ3R2kvCjd2UU1mWGRHc1JyWE5HUkduWCt2V0RaMy96V0kwam9EdENrTm5xRXBWbi4uSG9YCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="

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
			Name: "all required fields set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.NutanixPrismCentralEndpointSpec{
						Host:     "prism-central.nutanix.com",
						Port:     9441,
						Insecure: true,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
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
						gomega.HaveKeyWithValue("insecure", true),
						gomega.HaveKey("credentialRef"),
						gomega.Not(gomega.HaveKey("additionalTrustBundle")),
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "additional trust bundle is set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.NutanixPrismCentralEndpointSpec{
						Host:     "prism-central.nutanix.com",
						Port:     9441,
						Insecure: true,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
						AdditionalTrustBundle: ptr.To(testCertBundle),
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
						// Assert the insecure field was set to false as the additional trust bundle is set
						gomega.HaveKeyWithValue("insecure", false),
						gomega.HaveKey("credentialRef"),
						gomega.HaveKey("additionalTrustBundle"),
					),
				},
			},
		},
	)
}
