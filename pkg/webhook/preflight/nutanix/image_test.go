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

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestVMImageCheck(t *testing.T) {
	testCases := []struct {
		name           string
		v4client       v4client
		machineDetails *carenv1.NutanixMachineDetails
		want           preflight.CheckResult
	}{
		{
			name:     "imageLookup not yet supported",
			v4client: &mockv4client{},
			machineDetails: &carenv1.NutanixMachineDetails{
				ImageLookup: &capxv1.NutanixImageLookup{
					Format: ptr.To("test-format"),
					BaseOS: "test-baseos",
				},
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: true,
				Warnings: []string{
					"test-field uses imageLookup, which is not yet supported by checks",
				},
			},
		},
		{
			name: "image found by uuid",
			v4client: &mockv4client{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					assert.Equal(t, "test-uuid", *uuid)
					resp := &vmmv4.GetImageApiResponse{}
					err := resp.SetData(vmmv4.Image{
						ObjectType_: ptr.To("vmm.v4.content.Image"),
						ExtId:       ptr.To("test-uuid"),
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
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: true,
			},
		},
		{
			name: "image found by name",
			v4client: &mockv4client{
				listImagesFunc: func(page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*vmmv4.ListImagesApiResponse,
					error,
				) {
					resp := &vmmv4.ListImagesApiResponse{}
					err := resp.SetData([]vmmv4.Image{
						{
							Name: ptr.To("test-image-name"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To("test-image-name"),
				},
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: true,
			},
		},
		{
			name: "image not found by name",
			v4client: &mockv4client{
				listImagesFunc: func(page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*vmmv4.ListImagesApiResponse,
					error,
				) {
					return &vmmv4.ListImagesApiResponse{}, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To("test-non-existent-image"),
				},
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: "expected to find 1 VM Image, found 0",
						Field:   "test-field",
					},
				},
			},
		},
		{
			name: "multiple images found by name",
			v4client: &mockv4client{
				listImagesFunc: func(page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*vmmv4.ListImagesApiResponse,
					error,
				) {
					resp := &vmmv4.ListImagesApiResponse{}
					err := resp.SetData([]vmmv4.Image{
						{
							Name: ptr.To("test-duplicate-image"),
						},
						{
							Name: ptr.To("test-duplicate-image"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To("test-duplicate-image"),
				},
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: "expected to find 1 VM Image, found 2",
						Field:   "test-field",
					},
				},
			},
		},
		{
			name: "error getting image by id",
			v4client: &mockv4client{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: ptr.To("test-uuid"),
				},
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "failed to get VM Image: api error",
						Field:   "test-field",
					},
				},
			},
		},
		{
			name: "error listing images",
			v4client: &mockv4client{
				listImagesFunc: func(page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*vmmv4.ListImagesApiResponse,
					error,
				) {
					return nil, fmt.Errorf("api error")
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To("test-image"),
				},
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "failed to get VM Image: api error",
						Field:   "test-field",
					},
				},
			},
		},
		{
			name:           "neither image nor imageLookup specified",
			v4client:       &mockv4client{},
			machineDetails: &carenv1.NutanixMachineDetails{
				// both Image and ImageLookup are nil
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testr.New(t)

			checker := &nutanixChecker{
				log:      logger,
				v4client: tc.v4client,
			}

			// Create the check
			checkFn := vmImageCheck(
				checker,
				tc.machineDetails,
				"test-field",
			)

			// Execute the check
			got := checkFn(context.Background())

			// Verify the result
			assert.Equal(t, tc.want.Name, got.Name)
			assert.Equal(t, tc.want.Allowed, got.Allowed)
			assert.Equal(t, tc.want.Error, got.Error)
			assert.Equal(t, tc.want.Causes, got.Causes)
		})
	}
}

func TestGetVMImages(t *testing.T) {
	testCases := []struct {
		name     string
		client   *mockv4client
		id       *capxv1.NutanixResourceIdentifier
		want     []vmmv4.Image
		wantErr  bool
		errorMsg string
	}{
		{
			name: "get image by uuid success",
			client: &mockv4client{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					assert.Equal(t, "test-uuid", *uuid)
					resp := &vmmv4.GetImageApiResponse{}
					err := resp.SetData(vmmv4.Image{
						ObjectType_: ptr.To("vmm.v4.content.Image"),
						ExtId:       ptr.To("test-uuid"),
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid"),
			},
			want: []vmmv4.Image{
				{
					ObjectType_: ptr.To("vmm.v4.content.Image"),
					ExtId:       ptr.To("test-uuid"),
				},
			},
			wantErr: false,
		},
		{
			name: "get image by name success",
			client: &mockv4client{
				listImagesFunc: func(page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*vmmv4.ListImagesApiResponse,
					error,
				) {
					assert.NotNil(t, filter)
					assert.Equal(t, "name eq 'test-name'", *filter)
					resp := &vmmv4.ListImagesApiResponse{}
					err := resp.SetData([]vmmv4.Image{
						{
							Name: ptr.To("test-name"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-name"),
			},
			want: []vmmv4.Image{
				{
					Name: ptr.To("test-name"),
				},
			},
			wantErr: false,
		},
		{
			name: "get image by uuid error",
			client: &mockv4client{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid"),
			},
			wantErr:  true,
			errorMsg: "api error",
		},
		{
			name: "get image by name error",
			client: &mockv4client{
				listImagesFunc: func(page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*vmmv4.ListImagesApiResponse,
					error,
				) {
					return nil, fmt.Errorf("api error")
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-name"),
			},
			wantErr:  true,
			errorMsg: "api error",
		},
		{
			name:   "neither name nor uuid specified",
			client: &mockv4client{},
			id:     &capxv1.NutanixResourceIdentifier{
				// Both Name and UUID are not set
			},
			wantErr:  true,
			errorMsg: "image identifier is missing both name and uuid",
		},
		{
			name: "invalid data from GetImageById",
			client: &mockv4client{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					return &vmmv4.GetImageApiResponse{
						Data: &vmmv4.OneOfGetImageApiResponseData{
							ObjectType_: ptr.To("wrong-type"),
						},
					}, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid"),
			},
			wantErr:  true,
			errorMsg: "failed to get data returned by GetImageById",
		},
		{
			name: "empty response from ListImages",
			client: &mockv4client{
				listImagesFunc: func(page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*vmmv4.ListImagesApiResponse,
					error,
				) {
					return &vmmv4.ListImagesApiResponse{
						Data: nil, // Empty data
					}, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-name"),
			},
			want:    []vmmv4.Image{},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getVMImages(tc.client, tc.id)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestInitVMImageChecks(t *testing.T) {
	testCases := []struct {
		name                                      string
		nutanixClusterConfigSpec                  *carenv1.NutanixClusterConfigSpec
		nutanixWorkerNodeConfigSpecByMDName       map[string]*carenv1.NutanixWorkerNodeConfigSpec
		v4client                                  v4client
		expectedChecks                            int
		expectedControlPlaneCheckFieldIncluded    bool
		expectedWorkerNodeCheckFieldPatternExists bool
	}{
		{
			name:                                   "client not initialized",
			nutanixClusterConfigSpec:               nil,
			nutanixWorkerNodeConfigSpecByMDName:    nil,
			v4client:                               nil,
			expectedChecks:                         0,
			expectedControlPlaneCheckFieldIncluded: false,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name:                                   "no nutanix configuration",
			nutanixClusterConfigSpec:               nil,
			nutanixWorkerNodeConfigSpecByMDName:    nil,
			v4client:                               &mockv4client{},
			expectedChecks:                         0,
			expectedControlPlaneCheckFieldIncluded: false,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name: "control plane configuration only",
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
			nutanixWorkerNodeConfigSpecByMDName:       nil,
			v4client:                                  &mockv4client{},
			expectedChecks:                            1,
			expectedControlPlaneCheckFieldIncluded:    true,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name:                     "worker nodes configuration only",
			nutanixClusterConfigSpec: nil,
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Image: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker-image"),
							},
						},
					},
				},
			},
			v4client:                               &mockv4client{},
			expectedChecks:                         1,
			expectedControlPlaneCheckFieldIncluded: false,
			expectedWorkerNodeCheckFieldPatternExists: true,
		},
		{
			name: "both control plane and worker nodes configuration",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Image: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierUUID,
								UUID: ptr.To("cp-uuid"),
							},
						},
					},
				},
			},
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Image: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker1-image"),
							},
						},
					},
				},
				"worker-2": {
					Nutanix: &carenv1.NutanixNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Image: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker2-image"),
							},
						},
					},
				},
			},
			v4client:                               &mockv4client{},
			expectedChecks:                         3, // 1 control plane + 2 workers
			expectedControlPlaneCheckFieldIncluded: true,
			expectedWorkerNodeCheckFieldPatternExists: true,
		},
		{
			name:                     "worker with nil Nutanix config",
			nutanixClusterConfigSpec: nil,
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: nil,
				},
				"worker-2": {
					Nutanix: &carenv1.NutanixNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Image: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker2-image"),
							},
						},
					},
				},
			},
			v4client:                               &mockv4client{},
			expectedChecks:                         1, // only worker-2
			expectedControlPlaneCheckFieldIncluded: false,
			expectedWorkerNodeCheckFieldPatternExists: true,
		},
		{
			name: "control plane with nil Nutanix config",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: nil, // null nutanix config
				},
			},
			nutanixWorkerNodeConfigSpecByMDName:       nil,
			expectedChecks:                            0,
			expectedControlPlaneCheckFieldIncluded:    false,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name: "null control plane config",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: nil, // null control plane
			},
			nutanixWorkerNodeConfigSpecByMDName:       nil,
			v4client:                                  &mockv4client{},
			expectedChecks:                            0,
			expectedControlPlaneCheckFieldIncluded:    false,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testr.New(t)
			checker := &nutanixChecker{
				log:                      logger,
				nutanixClusterConfigSpec: tc.nutanixClusterConfigSpec,
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: tc.nutanixWorkerNodeConfigSpecByMDName,
				v4client: tc.v4client,
			}

			// Trap the vmImageCheck calls to verify field paths
			var capturedFields []string
			checker.vmImageCheckFunc = func(
				n *nutanixChecker,
				machineDetails *carenv1.NutanixMachineDetails,
				field string,
			) preflight.Check {
				capturedFields = append(capturedFields, field)
				return func(ctx context.Context) preflight.CheckResult {
					// Simulate a successful check
					return preflight.CheckResult{
						Name:    "NutanixVMImage",
						Allowed: true,
					}
				}
			}

			// Call the method under test
			checker.initVMImageChecksFunc = initVMImageChecks
			checks := checker.initVMImageChecksFunc(checker)

			// Verify number of checks
			assert.Len(t, checks, tc.expectedChecks)
			assert.Len(t, capturedFields, tc.expectedChecks)

			// Verify field names in checks
			if tc.expectedControlPlaneCheckFieldIncluded {
				assert.Contains(
					t,
					capturedFields,
					"cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix.machineDetails",
				)
			}

			if tc.expectedWorkerNodeCheckFieldPatternExists {
				foundWorkerFieldPattern := false
				for _, field := range capturedFields {
					if field != "cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix.machineDetails" {
						// This is not the control plane field, so it must be a worker node field
						assert.Contains(t, field, "cluster.spec.topology.workers.machineDeployments[.name=")
						assert.Contains(t, field, "].variables[.name=workerConfig].value.nutanix.machineDetails")
						foundWorkerFieldPattern = true
					}
				}
				assert.True(t, foundWorkerFieldPattern, "Worker node field pattern not found in any checks")
			}
		})
	}
}
