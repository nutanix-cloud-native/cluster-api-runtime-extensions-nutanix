// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestExtractKubernetesVersionFromImageName(t *testing.T) {
	testCases := []struct {
		name      string
		imageName string
		want      string
		wantErr   bool
	}{
		{
			name:      "kubedistro ubuntu vgpu image",
			imageName: "kubedistro-ubuntu-22.04-vgpu-1.32.3-20250604180644",
			want:      "1.32.3",
			wantErr:   false,
		},
		{
			name:      "kubedistro rocky release cis image",
			imageName: "kubedistro-rocky-9.5-release-cis-1.32.3-20250430150550",
			want:      "1.32.3",
			wantErr:   false,
		},
		{
			name:      "kubedistro rhel fips image",
			imageName: "kubedistro-rhel-8.10-fips-1.32.3-20250505212227",
			want:      "1.32.3",
			wantErr:   false,
		},
		{
			name:      "kubedistro rocky basic image",
			imageName: "kubedistro-rocky-9.5-1.32.3-20250514222748",
			want:      "1.32.3",
			wantErr:   false,
		},
		{
			name:      "different k8s version",
			imageName: "kubedistro-ubuntu-22.04-1.31.5-20250101000000",
			want:      "1.31.5",
			wantErr:   false,
		},
		{
			name:      "custom image name with kubernetes-version at end", // e.g., not following kubedistro naming convention
			imageName: "custom-image-v1.23",
			want:      "1.23",
			wantErr:   false,
		},
		{
			name:      "custom image name with kubernetes version in middle", // e.g., not following kubedistro naming convention
			imageName: "custom-v1.23.1-image",
			want:      "1.23.1",
			wantErr:   false,
		},
		{
			name:      "custom image name with kubernetes version in start", // e.g., not following kubedistro naming convention
			imageName: "v1.23.1-alpha-custom-image",
			want:      "1.23.1",
			wantErr:   false,
		},
		{
			name:      "custom image name - no match", // e.g., not following kubedistro naming convention
			imageName: "my-custom-image-name",
			want:      "",
			wantErr:   true,
		},
		{
			name:      "empty image name",
			imageName: "",
			want:      "",
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractKubernetesVersionFromImageName(tc.imageName)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "image name does not match expected naming convention")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestVMImageCheckWithKubernetesVersion(t *testing.T) {
	testCases := []struct {
		name              string
		nclient           client
		machineDetails    *carenv1.NutanixMachineDetails
		clusterK8sVersion string
		want              preflight.CheckResult
	}{
		{
			name: "kubernetes version matches",
			nclient: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					resp := &vmmv4.GetImageApiResponse{}
					err := resp.SetData(vmmv4.Image{
						ObjectType_: ptr.To("vmm.v4.content.Image"),
						ExtId:       ptr.To("test-uuid"),
						Name:        ptr.To("kubedistro-ubuntu-22.04-vgpu-1.32.3-20250604180644"),
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: ptr.To("test-uuid"),
				},
			},
			clusterK8sVersion: "1.32.3",
			want: preflight.CheckResult{
				Allowed: true,
			},
		},
		{
			name: "kubernetes version mismatch",
			nclient: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					resp := &vmmv4.GetImageApiResponse{}
					err := resp.SetData(vmmv4.Image{
						ObjectType_: ptr.To("vmm.v4.content.Image"),
						ExtId:       ptr.To("test-uuid"),
						Name:        ptr.To("kubedistro-ubuntu-22.04-vgpu-1.31.5-20250604180644"),
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: ptr.To("test-uuid"),
				},
			},
			clusterK8sVersion: "1.32.3",
			want: preflight.CheckResult{
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "kubernetes version mismatch: cluster kubernetes version '1.32.3' does not match image kubernetes version '1.31.5' (from image name 'kubedistro-ubuntu-22.04-vgpu-1.31.5-20250604180644')", //nolint:lll // cause is long
						Field:   "test-field",
					},
				},
			},
		},
		{
			name: "custom image name - extraction fails",
			nclient: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					resp := &vmmv4.GetImageApiResponse{}
					err := resp.SetData(vmmv4.Image{
						ObjectType_: ptr.To("vmm.v4.content.Image"),
						ExtId:       ptr.To("test-uuid"),
						Name:        ptr.To("my-custom-image-name"),
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: ptr.To("test-uuid"),
				},
			},
			clusterK8sVersion: "1.32.3",
			want: preflight.CheckResult{
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "failed to extract Kubernetes version from image name 'my-custom-image-name': image name does not match expected naming convention (expected pattern: .*<k8s-version>.*). This check assumes a naming convention that includes kubernetes version in the name. You can opt out of this check if using non-compliant naming", //nolint:lll // cause is long
						Field:   "test-field",
					},
				},
			},
		},
		{
			name: "empty image name",
			nclient: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					resp := &vmmv4.GetImageApiResponse{}
					err := resp.SetData(vmmv4.Image{
						ObjectType_: ptr.To("vmm.v4.content.Image"),
						ExtId:       ptr.To("test-uuid"),
						Name:        nil, // empty name
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: ptr.To("test-uuid"),
				},
			},
			clusterK8sVersion: "1.32.3",
			want: preflight.CheckResult{
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "VM image name is empty",
						Field:   "test-field",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			check := &imageKubernetesVersionCheck{
				machineDetails:    tc.machineDetails,
				field:             "test-field",
				nclient:           tc.nclient,
				clusterK8sVersion: tc.clusterK8sVersion,
			}

			got := check.Run(context.Background())

			assert.Equal(t, tc.want.Allowed, got.Allowed)
			assert.Equal(t, tc.want.Error, got.Error)
			assert.Equal(t, tc.want.Causes, got.Causes)
		})
	}
}

func TestNewVMImageChecksWithKubernetesVersion(t *testing.T) {
	testCases := []struct {
		name            string
		cluster         *clusterv1.Cluster
		expectedVersion string
		expectedChecks  int
	}{
		{
			name: "cluster with topology version",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Version: "v1.32.3",
					},
				},
			},
			expectedVersion: "1.32.3",
			expectedChecks:  1,
		},
		{
			name: "cluster with topology version without v prefix",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Version: "1.32.3",
					},
				},
			},
			expectedVersion: "1.32.3",
			expectedChecks:  1,
		},
		{
			name: "cluster without topology",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: nil,
				},
			},
			expectedVersion: "",
			expectedChecks:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cd := &checkDependencies{
				cluster: tc.cluster,
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					ControlPlane: &carenv1.NutanixControlPlaneSpec{
						Nutanix: &carenv1.NutanixNodeSpec{
							MachineDetails: carenv1.NutanixMachineDetails{
								Image: &capxv1.NutanixResourceIdentifier{
									Type: capxv1.NutanixIdentifierUUID,
									UUID: ptr.To("test-uuid"),
								},
							},
						},
					},
				},
				nclient: &mocknclient{},
				log:     testr.New(t),
			}

			checks := newVMImageKubernetesVersionChecks(cd)

			require.Len(t, checks, tc.expectedChecks, "unexpected number of checks created")
			if tc.expectedChecks != 0 {
				check := checks[0].(*imageKubernetesVersionCheck)
				assert.Equal(t, tc.expectedVersion, check.clusterK8sVersion)
			}
		})
	}
}
