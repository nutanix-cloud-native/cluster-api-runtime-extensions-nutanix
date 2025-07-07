// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
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
				Allowed:       false,
				InternalError: false,
				Causes: []preflight.Cause{
					{
						Message: "Kubernetes version check failed: kubernetes version \"1.32.3\" is not part " +
							"of image name \"kubedistro-ubuntu-22.04-vgpu-1.31.5-20250604180644\"",
						Field: "machineDetails.image",
					},
				},
			},
		},
		{
			name: "kubernetes version with build metadata matches",
			nclient: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					resp := &vmmv4.GetImageApiResponse{}
					err := resp.SetData(vmmv4.Image{
						ObjectType_: ptr.To("vmm.v4.content.Image"),
						ExtId:       ptr.To("test-uuid"),
						Name:        ptr.To("kubedistro-rhel-8.10-release-fips-1.33.1-20250704023459"),
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
			clusterK8sVersion: "1.33.1+fips.0",
			want: preflight.CheckResult{
				Allowed: true,
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
				Allowed:       false,
				InternalError: false,
				Causes: []preflight.Cause{
					{
						Message: "Kubernetes version check failed: kubernetes version \"1.32.3\" is not part of " +
							"image name \"my-custom-image-name\"",
						Field: "machineDetails.image",
					},
				},
			},
		},
		{
			name: "invalid kubernetes version",
			nclient: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					resp := &vmmv4.GetImageApiResponse{}
					err := resp.SetData(vmmv4.Image{
						ObjectType_: ptr.To("vmm.v4.content.Image"),
						ExtId:       ptr.To("test-uuid"),
						Name:        ptr.To("kubedistro-rhel-8.10-release-fips-1.33.1-20250704023459"),
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
			clusterK8sVersion: "invalid.version",
			want: preflight.CheckResult{
				Allowed:       false,
				InternalError: false,
				Causes: []preflight.Cause{
					{
						Message: "Kubernetes version check failed: failed to parse kubernetes version " +
							"\"invalid.version\": No Major.Minor.Patch elements found",
						Field: "machineDetails.image",
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
				Allowed:       false,
				InternalError: false,
				Causes: []preflight.Cause{
					{
						Message: "Kubernetes version check failed: VM image name is empty",
						Field:   "machineDetails.image",
					},
				},
			},
		},
		{
			name:    "imageLookup not yet supported",
			nclient: &mocknclient{},
			machineDetails: &carenv1.NutanixMachineDetails{
				ImageLookup: &capxv1.NutanixImageLookup{
					Format: ptr.To("test-format"),
					BaseOS: "test-baseos",
				},
			},
			want: preflight.CheckResult{
				Allowed: true,
				Warnings: []string{
					"Field test-field uses imageLookup, which is not yet supported by checks",
				},
			},
		},
		{
			name: "no images found",
			nclient: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					return nil, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: ptr.To("test-uuid"),
				},
			},
			want: preflight.CheckResult{
				Allowed: true,
			},
		},
		{
			name: "error getting images",
			nclient: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					return nil, fmt.Errorf("some error")
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: ptr.To("test-uuid"),
				},
			},
			want: preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "Failed to get VM Image: some error",
						Field:   "machineDetails.image",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			check := &imageKubernetesVersionCheck{
				machineDetails:    tc.machineDetails,
				field:             "machineDetails",
				nclient:           tc.nclient,
				clusterK8sVersion: tc.clusterK8sVersion,
			}

			got := check.Run(context.Background())

			assert.Equal(t, tc.want.Allowed, got.Allowed)
			assert.Equal(t, tc.want.InternalError, got.InternalError)
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
		nclient         client
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
			expectedChecks:  2,
			nclient:         &mocknclient{},
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
			expectedChecks:  2,
			nclient:         &mocknclient{},
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
			nclient:         &mocknclient{},
		},
		{
			name: "client not initialized",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Version: "v1.32.3",
					},
				},
			},
			expectedVersion: "1.32.3",
			expectedChecks:  0,
			nclient:         nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cd := &checkDependencies{
				cluster: tc.cluster,
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					ControlPlane: &carenv1.NutanixControlPlaneSpec{
						Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
							MachineDetails: carenv1.NutanixMachineDetails{
								Image: &capxv1.NutanixResourceIdentifier{
									Type: capxv1.NutanixIdentifierUUID,
									UUID: ptr.To("test-uuid"),
								},
							},
						},
					},
				},
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
					"test-md": {
						Nutanix: &carenv1.NutanixWorkerNodeSpec{
							MachineDetails: carenv1.NutanixMachineDetails{
								Image: &capxv1.NutanixResourceIdentifier{
									Type: capxv1.NutanixIdentifierUUID,
									UUID: ptr.To("test-uuid"),
								},
							},
						},
					},
				},
				nclient: tc.nclient,
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
