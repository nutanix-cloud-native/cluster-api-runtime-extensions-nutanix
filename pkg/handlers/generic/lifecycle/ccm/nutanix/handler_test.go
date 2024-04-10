// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	in = `---
prismCentralEndPoint: {{ .PrismCentralHost }}
prismCentralPort: {{ .PrismCentralPort }}
prismCentralInsecure: {{ .PrismCentralInsecure }}
prismCentralAdditionalTrustBundle: "{{ or .PrismCentralAdditionalTrustBundle "" }}"

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
`
	//nolint:lll // just a long string
	testCertBundle = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUVjekNDQTF1Z0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRUUZBRC4uQWtHQTFVRUJoTUNSMEl4CkV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGREFTQmdOVkJBb1RDMC4uMEVnVEhSa01UY3dOUVlEClZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbWx0WVhKNUlFTmxjbi4uWFJwYjI0Z1FYVjBhRzl5CmFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRXgwWkRBZUZ3MHdNRC4uVFV3TVRaYUZ3MHdNVEF5Ck1EUXhPVFV3TVRaYU1JR0hNUXN3Q1FZRFZRUUdFd0pIUWpFVE1CRUdBMS4uMjl0WlMxVGRHRjBaVEVVCk1CSUdBMVVFQ2hNTFFtVnpkQ0JEUVNCTWRHUXhOekExQmdOVkJBc1RMay4uREVnVUhWaWJHbGpJRkJ5CmFXMWhjbmtnUTJWeWRHbG1hV05oZEdsdmJpQkJkWFJvYjNKcGRIa3hGRC4uQU1UQzBKbGMzUWdRMEVnClRIUmtNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZy4uVHoybXI3U1ppQU1mUXl1CnZCak05T2lKalJhelhCWjFCalA1Q0UvV20vUnI1MDBQUksrTGg5eDVlSi4uL0FOQkUwc1RLMFpzREdNCmFrMm0xZzdvcnVJM2RZM1ZIcUl4RlR6MFRhMWQrTkFqd25MZTRuT2I3Ly4uazA1U2hoQnJKR0JLS3hiCjhuMTA0by81cDhIQXNaUGR6YkZNSXlOakp6Qk0ybzV5NUExM3dpTGl0RS4uZnlZa1F6YXhDdzBBd3psCmtWSGlJeUN1YUY0d2o1NzFwU3prdjZzdis0SURNYlQvWHBDbzhMNndUYS4uc2grZXRMRDZGdFRqWWJiCnJ2WjhSUU0xdGxLZG9NSGcycXhyYUFWKytITkJZbU5XczBkdUVkalViSi4uWEk5VHRuUzRvMUNrajdQCk9mbGppUUlEQVFBQm80SG5NSUhrTUIwR0ExVWREZ1FXQkJROHVyTUNSTC4uNUFrSXA5TkpISnc1VENCCnRBWURWUjBqQklHc01JR3BnQlE4dXJNQ1JMWVlNSFVLVTVBa0lwOU5KSC4uYVNCaWpDQmh6RUxNQWtHCkExVUVCaE1DUjBJeEV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGRC4uQW9UQzBKbGMzUWdRMEVnClRIUmtNVGN3TlFZRFZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbS4uRU5sY25ScFptbGpZWFJwCmIyNGdRWFYwYUc5eWFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRS4uREFNQmdOVkhSTUVCVEFECkFRSC9NQTBHQ1NxR1NJYjNEUUVCQkFVQUE0SUJBUUMxdVlCY3NTbmN3QS4uRENzUWVyNzcyQzJ1Y3BYCnhRVUUvQzBwV1dtNmdEa3dkNUQwRFNNREpScVYvd2VvWjR3QzZCNzNmNS4uYkxoR1lIYVhKZVNENktyClhjb093TGRTYUdtSllzbExLWkIzWklERXAwd1lUR2hndGViNkpGaVR0bi4uc2YyeGRyWWZQQ2lJQjdnCkJNQVY3R3pkYzRWc3BTNmxqckFoYmlpYXdkQmlRbFFtc0JlRno5SmtGNC4uYjNsOEJvR04rcU1hNTZZCkl0OHVuYTJnWTRsMk8vL29uODhyNUlXSmxtMUwwb0E4ZTRmUjJ5ckJIWC4uYWRzR2VGS2t5TnJ3R2kvCjd2UU1mWGRHc1JyWE5HUkduWCt2V0RaMy96V0kwam9EdENrTm5xRXBWbi4uSG9YCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="
)

const (
	//nolint:lll // just a long string
	expectedWithAdditionalTrustBundle = `---
prismCentralEndPoint: prism-central.nutanix.com
prismCentralPort: 9440
prismCentralInsecure: false
prismCentralAdditionalTrustBundle: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUVjekNDQTF1Z0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRUUZBRC4uQWtHQTFVRUJoTUNSMEl4CkV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGREFTQmdOVkJBb1RDMC4uMEVnVEhSa01UY3dOUVlEClZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbWx0WVhKNUlFTmxjbi4uWFJwYjI0Z1FYVjBhRzl5CmFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRXgwWkRBZUZ3MHdNRC4uVFV3TVRaYUZ3MHdNVEF5Ck1EUXhPVFV3TVRaYU1JR0hNUXN3Q1FZRFZRUUdFd0pIUWpFVE1CRUdBMS4uMjl0WlMxVGRHRjBaVEVVCk1CSUdBMVVFQ2hNTFFtVnpkQ0JEUVNCTWRHUXhOekExQmdOVkJBc1RMay4uREVnVUhWaWJHbGpJRkJ5CmFXMWhjbmtnUTJWeWRHbG1hV05oZEdsdmJpQkJkWFJvYjNKcGRIa3hGRC4uQU1UQzBKbGMzUWdRMEVnClRIUmtNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZy4uVHoybXI3U1ppQU1mUXl1CnZCak05T2lKalJhelhCWjFCalA1Q0UvV20vUnI1MDBQUksrTGg5eDVlSi4uL0FOQkUwc1RLMFpzREdNCmFrMm0xZzdvcnVJM2RZM1ZIcUl4RlR6MFRhMWQrTkFqd25MZTRuT2I3Ly4uazA1U2hoQnJKR0JLS3hiCjhuMTA0by81cDhIQXNaUGR6YkZNSXlOakp6Qk0ybzV5NUExM3dpTGl0RS4uZnlZa1F6YXhDdzBBd3psCmtWSGlJeUN1YUY0d2o1NzFwU3prdjZzdis0SURNYlQvWHBDbzhMNndUYS4uc2grZXRMRDZGdFRqWWJiCnJ2WjhSUU0xdGxLZG9NSGcycXhyYUFWKytITkJZbU5XczBkdUVkalViSi4uWEk5VHRuUzRvMUNrajdQCk9mbGppUUlEQVFBQm80SG5NSUhrTUIwR0ExVWREZ1FXQkJROHVyTUNSTC4uNUFrSXA5TkpISnc1VENCCnRBWURWUjBqQklHc01JR3BnQlE4dXJNQ1JMWVlNSFVLVTVBa0lwOU5KSC4uYVNCaWpDQmh6RUxNQWtHCkExVUVCaE1DUjBJeEV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGRC4uQW9UQzBKbGMzUWdRMEVnClRIUmtNVGN3TlFZRFZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbS4uRU5sY25ScFptbGpZWFJwCmIyNGdRWFYwYUc5eWFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRS4uREFNQmdOVkhSTUVCVEFECkFRSC9NQTBHQ1NxR1NJYjNEUUVCQkFVQUE0SUJBUUMxdVlCY3NTbmN3QS4uRENzUWVyNzcyQzJ1Y3BYCnhRVUUvQzBwV1dtNmdEa3dkNUQwRFNNREpScVYvd2VvWjR3QzZCNzNmNS4uYkxoR1lIYVhKZVNENktyClhjb093TGRTYUdtSllzbExLWkIzWklERXAwd1lUR2hndGViNkpGaVR0bi4uc2YyeGRyWWZQQ2lJQjdnCkJNQVY3R3pkYzRWc3BTNmxqckFoYmlpYXdkQmlRbFFtc0JlRno5SmtGNC4uYjNsOEJvR04rcU1hNTZZCkl0OHVuYTJnWTRsMk8vL29uODhyNUlXSmxtMUwwb0E4ZTRmUjJ5ckJIWC4uYWRzR2VGS2t5TnJ3R2kvCjd2UU1mWGRHc1JyWE5HUkduWCt2V0RaMy96V0kwam9EdENrTm5xRXBWbi4uSG9YCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
`

	expectedWithoutAdditionalTrustBundle = `---
prismCentralEndPoint: prism-central.nutanix.com
prismCentralPort: 9440
prismCentralInsecure: true
prismCentralAdditionalTrustBundle: ""

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
`
)

func Test_templateValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		clusterConfig *v1alpha1.ClusterConfigSpec
		in            string
		expected      string
	}{
		{
			name: "With AdditionalTrustBundle set",
			clusterConfig: &v1alpha1.ClusterConfigSpec{
				GenericClusterConfig: v1alpha1.GenericClusterConfig{
					Addons: &v1alpha1.Addons{
						CCM: &v1alpha1.CCM{
							Credentials: &corev1.LocalObjectReference{
								Name: "creds",
							},
						},
					},
				},
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:                   fmt.Sprintf("https://prism-central.nutanix.com:%d", v1alpha1.DefaultPrismCentralPort),
						AdditionalTrustBundle: ptr.To(testCertBundle),
					},
				},
			},
			in:       in,
			expected: expectedWithAdditionalTrustBundle,
		},
		{
			name: "Without an AdditionalTrustBundle set",
			clusterConfig: &v1alpha1.ClusterConfigSpec{
				GenericClusterConfig: v1alpha1.GenericClusterConfig{
					Addons: &v1alpha1.Addons{
						CCM: &v1alpha1.CCM{
							Credentials: &corev1.LocalObjectReference{
								Name: "creds",
							},
						},
					},
				},
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      fmt.Sprintf("https://prism-central.nutanix.com:%d", v1alpha1.DefaultPrismCentralPort),
						Insecure: true,
					},
				},
			},
			in:       in,
			expected: expectedWithoutAdditionalTrustBundle,
		},
	}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := templateValues(tt.clusterConfig, tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}
