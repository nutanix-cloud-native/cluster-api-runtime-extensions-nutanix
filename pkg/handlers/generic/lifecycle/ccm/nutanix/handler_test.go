// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	//nolint:lll // just a long string
	testCertBundle = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUVjekNDQTF1Z0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRUUZBRC4uQWtHQTFVRUJoTUNSMEl4CkV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGREFTQmdOVkJBb1RDMC4uMEVnVEhSa01UY3dOUVlEClZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbWx0WVhKNUlFTmxjbi4uWFJwYjI0Z1FYVjBhRzl5CmFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRXgwWkRBZUZ3MHdNRC4uVFV3TVRaYUZ3MHdNVEF5Ck1EUXhPVFV3TVRaYU1JR0hNUXN3Q1FZRFZRUUdFd0pIUWpFVE1CRUdBMS4uMjl0WlMxVGRHRjBaVEVVCk1CSUdBMVVFQ2hNTFFtVnpkQ0JEUVNCTWRHUXhOekExQmdOVkJBc1RMay4uREVnVUhWaWJHbGpJRkJ5CmFXMWhjbmtnUTJWeWRHbG1hV05oZEdsdmJpQkJkWFJvYjNKcGRIa3hGRC4uQU1UQzBKbGMzUWdRMEVnClRIUmtNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZy4uVHoybXI3U1ppQU1mUXl1CnZCak05T2lKalJhelhCWjFCalA1Q0UvV20vUnI1MDBQUksrTGg5eDVlSi4uL0FOQkUwc1RLMFpzREdNCmFrMm0xZzdvcnVJM2RZM1ZIcUl4RlR6MFRhMWQrTkFqd25MZTRuT2I3Ly4uazA1U2hoQnJKR0JLS3hiCjhuMTA0by81cDhIQXNaUGR6YkZNSXlOakp6Qk0ybzV5NUExM3dpTGl0RS4uZnlZa1F6YXhDdzBBd3psCmtWSGlJeUN1YUY0d2o1NzFwU3prdjZzdis0SURNYlQvWHBDbzhMNndUYS4uc2grZXRMRDZGdFRqWWJiCnJ2WjhSUU0xdGxLZG9NSGcycXhyYUFWKytITkJZbU5XczBkdUVkalViSi4uWEk5VHRuUzRvMUNrajdQCk9mbGppUUlEQVFBQm80SG5NSUhrTUIwR0ExVWREZ1FXQkJROHVyTUNSTC4uNUFrSXA5TkpISnc1VENCCnRBWURWUjBqQklHc01JR3BnQlE4dXJNQ1JMWVlNSFVLVTVBa0lwOU5KSC4uYVNCaWpDQmh6RUxNQWtHCkExVUVCaE1DUjBJeEV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGRC4uQW9UQzBKbGMzUWdRMEVnClRIUmtNVGN3TlFZRFZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbS4uRU5sY25ScFptbGpZWFJwCmIyNGdRWFYwYUc5eWFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRS4uREFNQmdOVkhSTUVCVEFECkFRSC9NQTBHQ1NxR1NJYjNEUUVCQkFVQUE0SUJBUUMxdVlCY3NTbmN3QS4uRENzUWVyNzcyQzJ1Y3BYCnhRVUUvQzBwV1dtNmdEa3dkNUQwRFNNREpScVYvd2VvWjR3QzZCNzNmNS4uYkxoR1lIYVhKZVNENktyClhjb093TGRTYUdtSllzbExLWkIzWklERXAwd1lUR2hndGViNkpGaVR0bi4uc2YyeGRyWWZQQ2lJQjdnCkJNQVY3R3pkYzRWc3BTNmxqckFoYmlpYXdkQmlRbFFtc0JlRno5SmtGNC4uYjNsOEJvR04rcU1hNTZZCkl0OHVuYTJnWTRsMk8vL29uODhyNUlXSmxtMUwwb0E4ZTRmUjJ5ckJIWC4uYWRzR2VGS2t5TnJ3R2kvCjd2UU1mWGRHc1JyWE5HUkduWCt2V0RaMy96V0kwam9EdENrTm5xRXBWbi4uSG9YCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="

	//nolint:lll // just a long string
	expectedWithAdditionalTrustBundle = `prismCentralEndPoint: prism-central.nutanix.com
prismCentralPort: 9440
prismCentralInsecure: false
prismCentralAdditionalTrustBundle: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUVjekNDQTF1Z0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRUUZBRC4uQWtHQTFVRUJoTUNSMEl4CkV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGREFTQmdOVkJBb1RDMC4uMEVnVEhSa01UY3dOUVlEClZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbWx0WVhKNUlFTmxjbi4uWFJwYjI0Z1FYVjBhRzl5CmFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRXgwWkRBZUZ3MHdNRC4uVFV3TVRaYUZ3MHdNVEF5Ck1EUXhPVFV3TVRaYU1JR0hNUXN3Q1FZRFZRUUdFd0pIUWpFVE1CRUdBMS4uMjl0WlMxVGRHRjBaVEVVCk1CSUdBMVVFQ2hNTFFtVnpkQ0JEUVNCTWRHUXhOekExQmdOVkJBc1RMay4uREVnVUhWaWJHbGpJRkJ5CmFXMWhjbmtnUTJWeWRHbG1hV05oZEdsdmJpQkJkWFJvYjNKcGRIa3hGRC4uQU1UQzBKbGMzUWdRMEVnClRIUmtNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZy4uVHoybXI3U1ppQU1mUXl1CnZCak05T2lKalJhelhCWjFCalA1Q0UvV20vUnI1MDBQUksrTGg5eDVlSi4uL0FOQkUwc1RLMFpzREdNCmFrMm0xZzdvcnVJM2RZM1ZIcUl4RlR6MFRhMWQrTkFqd25MZTRuT2I3Ly4uazA1U2hoQnJKR0JLS3hiCjhuMTA0by81cDhIQXNaUGR6YkZNSXlOakp6Qk0ybzV5NUExM3dpTGl0RS4uZnlZa1F6YXhDdzBBd3psCmtWSGlJeUN1YUY0d2o1NzFwU3prdjZzdis0SURNYlQvWHBDbzhMNndUYS4uc2grZXRMRDZGdFRqWWJiCnJ2WjhSUU0xdGxLZG9NSGcycXhyYUFWKytITkJZbU5XczBkdUVkalViSi4uWEk5VHRuUzRvMUNrajdQCk9mbGppUUlEQVFBQm80SG5NSUhrTUIwR0ExVWREZ1FXQkJROHVyTUNSTC4uNUFrSXA5TkpISnc1VENCCnRBWURWUjBqQklHc01JR3BnQlE4dXJNQ1JMWVlNSFVLVTVBa0lwOU5KSC4uYVNCaWpDQmh6RUxNQWtHCkExVUVCaE1DUjBJeEV6QVJCZ05WQkFnVENsTnZiV1V0VTNSaGRHVXhGRC4uQW9UQzBKbGMzUWdRMEVnClRIUmtNVGN3TlFZRFZRUUxFeTVEYkdGemN5QXhJRkIxWW14cFl5QlFjbS4uRU5sY25ScFptbGpZWFJwCmIyNGdRWFYwYUc5eWFYUjVNUlF3RWdZRFZRUURFd3RDWlhOMElFTkJJRS4uREFNQmdOVkhSTUVCVEFECkFRSC9NQTBHQ1NxR1NJYjNEUUVCQkFVQUE0SUJBUUMxdVlCY3NTbmN3QS4uRENzUWVyNzcyQzJ1Y3BYCnhRVUUvQzBwV1dtNmdEa3dkNUQwRFNNREpScVYvd2VvWjR3QzZCNzNmNS4uYkxoR1lIYVhKZVNENktyClhjb093TGRTYUdtSllzbExLWkIzWklERXAwd1lUR2hndGViNkpGaVR0bi4uc2YyeGRyWWZQQ2lJQjdnCkJNQVY3R3pkYzRWc3BTNmxqckFoYmlpYXdkQmlRbFFtc0JlRno5SmtGNC4uYjNsOEJvR04rcU1hNTZZCkl0OHVuYTJnWTRsMk8vL29uODhyNUlXSmxtMUwwb0E4ZTRmUjJ5ckJIWC4uYWRzR2VGS2t5TnJ3R2kvCjd2UU1mWGRHc1JyWE5HUkduWCt2V0RaMy96V0kwam9EdENrTm5xRXBWbi4uSG9YCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="
ignoredNodeIPs: [ "1.2.3.4" ]

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
`

	expectedWithoutAdditionalTrustBundle = `prismCentralEndPoint: prism-central.nutanix.com
prismCentralPort: 9440
prismCentralInsecure: true
ignoredNodeIPs: [ "1.2.3.4" ]

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
`

	expectedWithVirtualIPSet = `prismCentralEndPoint: prism-central.nutanix.com
prismCentralPort: 9440
prismCentralInsecure: true
ignoredNodeIPs: [ "1.2.3.4", "5.6.7.8" ]

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
`
)

var valuesTemplateFile = filepath.Join(
	moduleRootDir(),
	"charts",
	"cluster-api-runtime-extensions-nutanix",
	"addons",
	"ccm",
	"nutanix",
	"values-template.yaml",
)

func Test_templateValues(t *testing.T) {
	t.Parallel()

	valuesTemplate := readCCMValuesTemplateFromProjectHelmChart(t)

	tests := []struct {
		name          string
		clusterConfig *apivariables.ClusterConfigSpec

		in       string
		expected string
	}{
		{
			name: "With AdditionalTrustBundle set",
			clusterConfig: &apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						CCM: &v1alpha1.CCM{
							Credentials: &v1alpha1.CCMCredentials{
								SecretRef: v1alpha1.LocalObjectReference{
									Name: "creds",
								},
							},
						},
					},
				},
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: fmt.Sprintf(
							"https://prism-central.nutanix.com:%d",
							v1alpha1.DefaultPrismCentralPort,
						),
						AdditionalTrustBundle: testCertBundle,
					},
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "1.2.3.4",
					},
				},
			},
			in:       valuesTemplate,
			expected: expectedWithAdditionalTrustBundle,
		},
		{
			name: "Without an AdditionalTrustBundle set",
			clusterConfig: &apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						CCM: &v1alpha1.CCM{
							Credentials: &v1alpha1.CCMCredentials{
								SecretRef: v1alpha1.LocalObjectReference{
									Name: "creds",
								},
							},
						},
					},
				},
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: fmt.Sprintf(
							"https://prism-central.nutanix.com:%d",
							v1alpha1.DefaultPrismCentralPort,
						),
						Insecure: true,
					},
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "1.2.3.4",
					},
				},
			},
			in:       valuesTemplate,
			expected: expectedWithoutAdditionalTrustBundle,
		},
		{
			name: "With VirtualIP Set",
			clusterConfig: &apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						CCM: &v1alpha1.CCM{
							Credentials: &v1alpha1.CCMCredentials{
								SecretRef: v1alpha1.LocalObjectReference{
									Name: "creds",
								},
							},
						},
					},
				},
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: fmt.Sprintf(
							"https://prism-central.nutanix.com:%d",
							v1alpha1.DefaultPrismCentralPort,
						),
						Insecure: true,
					},
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "1.2.3.4",
						VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
							Configuration: &v1alpha1.ControlPlaneVirtualIPConfiguration{
								Address: "5.6.7.8",
							},
						},
					},
				},
			},
			in:       valuesTemplate,
			expected: expectedWithVirtualIPSet,
		},
	}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := templateValuesFunc(tt.clusterConfig.Nutanix)(nil, tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

// readCCMValuesTemplateFromProjectHelmChart reads the requested Helm values file from the Helm chart in the project.
// This is important to do this way to ensure that the hard-to-read double templating works as expected.
func readCCMValuesTemplateFromProjectHelmChart(t *testing.T) string {
	t.Helper()

	bs, err := os.ReadFile(valuesTemplateFile)
	require.NoError(t, err)
	return string(bs)
}

func moduleRootDir() string {
	cmd := exec.Command("go", "list", "-m", "-f", "{{ .Dir }}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// We include the combined output because the error is usually
		// an exit code, which does not explain why the command failed.
		panic(
			fmt.Sprintf("cmd.Dir=%q, cmd.Env=%q, cmd.Args=%q, err=%q, output=%q",
				cmd.Dir,
				cmd.Env,
				cmd.Args,
				err,
				out),
		)
	}
	return strings.TrimSpace(string(out))
}
