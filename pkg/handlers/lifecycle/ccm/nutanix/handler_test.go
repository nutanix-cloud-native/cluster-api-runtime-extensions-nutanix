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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

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
ignoredNodeIPs: ["1.2.3.4"]

# The Service LoadBalancer controller runs only in automatic mode, where the CCM
# reserves each Service address on Prism Central. In static mode MetalLB owns
# allocation, status and advertisement, so the controller stays off: running it
# as a no-op would still add the load-balancer-cleanup finalizer to every
# LoadBalancer Service, which existing clusters do not have today.
loadBalancer:
  enabled: false

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
extraEnv:
  - name: KUBERNETES_SERVICE_HOST
    value: "1.2.3.4"
  - name: KUBERNETES_SERVICE_PORT
    value: "6443"
`

	expectedWithoutAdditionalTrustBundle = `prismCentralEndPoint: prism-central.nutanix.com
prismCentralPort: 9440
prismCentralInsecure: true
ignoredNodeIPs: ["1.2.3.4"]

# The Service LoadBalancer controller runs only in automatic mode, where the CCM
# reserves each Service address on Prism Central. In static mode MetalLB owns
# allocation, status and advertisement, so the controller stays off: running it
# as a no-op would still add the load-balancer-cleanup finalizer to every
# LoadBalancer Service, which existing clusters do not have today.
loadBalancer:
  enabled: false

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
extraEnv:
  - name: KUBERNETES_SERVICE_HOST
    value: "1.2.3.4"
  - name: KUBERNETES_SERVICE_PORT
    value: "6443"
`

	expectedWithVirtualIPSet = `prismCentralEndPoint: prism-central.nutanix.com
prismCentralPort: 9440
prismCentralInsecure: true
ignoredNodeIPs: ["1.2.3.4", "5.6.7.8"]

# The Service LoadBalancer controller runs only in automatic mode, where the CCM
# reserves each Service address on Prism Central. In static mode MetalLB owns
# allocation, status and advertisement, so the controller stays off: running it
# as a no-op would still add the load-balancer-cleanup finalizer to every
# LoadBalancer Service, which existing clusters do not have today.
loadBalancer:
  enabled: false

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
extraEnv:
  - name: KUBERNETES_SERVICE_HOST
    value: "1.2.3.4"
  - name: KUBERNETES_SERVICE_PORT
    value: "6443"
`

	expectedWithAutoIPAM = `prismCentralEndPoint: prism-central.nutanix.com
prismCentralPort: 9440
prismCentralInsecure: true
ignoredNodeIPs: ["1.2.3.4"]

# The Service LoadBalancer controller runs only in automatic mode, where the CCM
# reserves each Service address on Prism Central. In static mode MetalLB owns
# allocation, status and advertisement, so the controller stays off: running it
# as a no-op would still add the load-balancer-cleanup finalizer to every
# LoadBalancer Service, which existing clusters do not have today.
loadBalancer:
  enabled: true
  defaultStrategy: AutoIPAMMetalLB
  autoIPAMMetalLB:
    subnet: "e3f4a1b2-0000-4000-8000-abcdefabcdef"
    clusterUID: "7c2d1e40-5f3a-4b21-9d8e-11aa22bb33cc"
    metalLBNamespace: "metallb-system"

# The Secret containing the credentials will be created by the handler.
createSecret: false
secretName: nutanix-ccm-credentials
extraEnv:
  - name: KUBERNETES_SERVICE_HOST
    value: "1.2.3.4"
  - name: KUBERNETES_SERVICE_PORT
    value: "6443"
`
)

// autoIPAMSubnet is the subnet UUID the automatic-mode cases reserve from.
const autoIPAMSubnet = "e3f4a1b2-0000-4000-8000-abcdefabcdef"

// expectedWithAutoIPAMSubnetName is expectedWithAutoIPAM with a subnet name and
// the PE cluster that resolves it.
var expectedWithAutoIPAMSubnetName = strings.Replace(
	expectedWithAutoIPAM,
	`    subnet: "`+autoIPAMSubnet+`"`,
	`    subnet: "vlan170-dhcp"`+"\n"+`    cluster: "pe-cluster-1"`,
	1,
)

// testClusterUID is the Cluster UID the rendered values must carry: it is the
// cluster half of the reservation context the CCM stores on Prism Central.
const testClusterUID = "7c2d1e40-5f3a-4b21-9d8e-11aa22bb33cc"

func testCluster() *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			UID:       types.UID(testClusterUID),
		},
	}
}

func serviceLoadBalancerAutoIPAM(subnet string, cluster *string) *v1alpha1.ServiceLoadBalancer {
	return &v1alpha1.ServiceLoadBalancer{
		Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
		Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
			AutoIPAM: &v1alpha1.ServiceLoadBalancerAutoIPAM{
				Subnet:  subnet,
				Cluster: cluster,
			},
		},
	}
}

// autoIPAMClusterConfig is the "Without an AdditionalTrustBundle set" cluster
// with the given ServiceLoadBalancer attached, so the rendered difference is
// only the loadBalancer block.
func autoIPAMClusterConfig(slb *v1alpha1.ServiceLoadBalancer) *apivariables.ClusterConfigSpec {
	return &apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			GenericAddons: v1alpha1.GenericAddons{
				CCM: &v1alpha1.CCM{
					Credentials: &v1alpha1.CCMCredentials{
						SecretRef: v1alpha1.LocalObjectReference{Name: "creds"},
					},
				},
				ServiceLoadBalancer: slb,
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
				Port: 6443,
			},
		},
	}
}

var valuesTemplateFile = func() string {
	dir, err := moduleRootDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(
		dir,
		"charts",
		"cluster-api-runtime-extensions-nutanix",
		"addons",
		"ccm",
		"nutanix",
		"values-template.yaml",
	)
}()

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
						Port: 6443,
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
						Port: 6443,
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
						Port: 6443,
						VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
							Configuration: &v1alpha1.ControlPlaneVirtualIPConfiguration{
								Address: "5.6.7.8",
								Port:    6443,
							},
						},
					},
				},
			},
			in:       valuesTemplate,
			expected: expectedWithVirtualIPSet,
		},
		{
			// Automatic mode: the CCM reserves each Service address, so the
			// Service controller is enabled and pointed at the subnet.
			name:          "With ServiceLoadBalancer autoIPAM subnet UUID",
			clusterConfig: autoIPAMClusterConfig(serviceLoadBalancerAutoIPAM(autoIPAMSubnet, nil)),
			in:            valuesTemplate,
			expected:      expectedWithAutoIPAM,
		},
		{
			// A subnet name needs the PE cluster that resolves it, matching how
			// NutanixIPPool identifies a subnet.
			name: "With ServiceLoadBalancer autoIPAM subnet name and cluster",
			clusterConfig: autoIPAMClusterConfig(
				serviceLoadBalancerAutoIPAM("vlan170-dhcp", new("pe-cluster-1")),
			),
			in:       valuesTemplate,
			expected: expectedWithAutoIPAMSubnetName,
		},
		{
			// Static ranges keep the controller off, same as no
			// ServiceLoadBalancer at all.
			name: "With ServiceLoadBalancer address ranges set",
			clusterConfig: autoIPAMClusterConfig(&v1alpha1.ServiceLoadBalancer{
				Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
				Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
					AddressRanges: []v1alpha1.AddressRange{
						{Start: "10.0.0.10", End: "10.0.0.20"},
					},
				},
			}),
			in:       valuesTemplate,
			expected: expectedWithoutAdditionalTrustBundle,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := templateValuesFunc(
				tt.clusterConfig.Nutanix,
				tt.clusterConfig.Addons.ServiceLoadBalancer,
			)(testCluster(), tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

// A cluster whose API server address is reserved from a subnet carries no host
// in its cluster config - the reserved address is resolved into the generated
// templates, never persisted - so the CCM must take the endpoint from the
// Cluster. Rendering an empty host instead sets KUBERNETES_SERVICE_HOST="",
// which stops the CCM falling back to the in-cluster config: it crash-loops,
// the nodes keep the uninitialized taint and nothing else can be scheduled.
func Test_templateValuesReservedControlPlaneEndpoint(t *testing.T) {
	t.Parallel()

	const reserved = "10.0.0.9"

	clusterConfig := autoIPAMClusterConfig(
		serviceLoadBalancerAutoIPAM("subnet", nil),
	)
	clusterConfig.Nutanix.ControlPlaneEndpoint = v1alpha1.ControlPlaneEndpointSpec{
		Port: 6443,
		VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
			Configuration: &v1alpha1.ControlPlaneVirtualIPConfiguration{
				IPReservation: &v1alpha1.ControlPlaneIPReservation{Subnet: "subnet"},
			},
		},
	}

	cluster := testCluster()
	cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{Host: reserved, Port: 6443}

	values, err := templateValuesFunc(clusterConfig.Nutanix, clusterConfig.Addons.ServiceLoadBalancer)(
		cluster, readCCMValuesTemplateFromProjectHelmChart(t),
	)
	require.NoError(t, err)

	assert.Contains(t, values, fmt.Sprintf("value: %q", reserved),
		"KUBERNETES_SERVICE_HOST must carry the reserved address")
	assert.NotContains(t, values, `value: ""`,
		"an empty env value defeats the CCM's in-cluster config fallback")
	assert.Contains(t, values, fmt.Sprintf("ignoredNodeIPs: [%q]", reserved),
		"the reserved API server address must not be published as a node address")
}

// Without a host anywhere the handler must fail rather than install a CCM that
// cannot reach the API server: the addon then retries and says why.
func Test_templateValuesNoControlPlaneEndpointFails(t *testing.T) {
	t.Parallel()

	clusterConfig := autoIPAMClusterConfig(nil)
	clusterConfig.Nutanix.ControlPlaneEndpoint = v1alpha1.ControlPlaneEndpointSpec{Port: 6443}

	_, err := templateValuesFunc(clusterConfig.Nutanix, nil)(
		testCluster(), readCCMValuesTemplateFromProjectHelmChart(t),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no control plane endpoint host")
}

// readCCMValuesTemplateFromProjectHelmChart reads the requested Helm values file from the Helm chart in the project.
// This is important to do this way to ensure that the hard-to-read double templating works as expected.
func readCCMValuesTemplateFromProjectHelmChart(t *testing.T) string {
	t.Helper()

	bs, err := os.ReadFile(valuesTemplateFile)
	require.NoError(t, err)
	return string(bs)
}

func moduleRootDir() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{ .Dir }}")
	out, err := cmd.CombinedOutput()
	if err != nil || len(out) == 0 {
		// We include the combined output because the error is usually
		// an exit code, which does not explain why the command failed.
		return "", fmt.Errorf("cmd.Dir=%q, cmd.Env=%q, cmd.Args=%q, err=%q, output=%q",
			cmd.Dir,
			cmd.Env,
			cmd.Args,
			err,
			out)
	}
	// The first line is the module root directory. When go workspaces are used,
	// the first line is the "root" module root directory.
	dir, _, _ := strings.Cut(string(out), "\n")
	return dir, nil
}
