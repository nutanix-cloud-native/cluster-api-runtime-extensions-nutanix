// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr/testr"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	vmmv4error "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/error"
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
		nclient        client
		machineDetails *carenv1.NutanixMachineDetails
		want           preflight.CheckResult
	}{
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
					"test-field uses imageLookup, which is not yet supported by checks",
				},
			},
		},
		{
			name: "image found by uuid",
			nclient: &mocknclient{
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
				Allowed: true,
			},
		},
		{
			name: "image found by name",
			nclient: &mocknclient{
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
				Allowed: true,
			},
		},
		{
			name: "image not found by name",
			nclient: &mocknclient{
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
			nclient: &mocknclient{
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
			nclient: &mocknclient{
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
				Allowed:       false,
				InternalError: true,
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
			nclient: &mocknclient{
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
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "failed to get VM Image: api error",
						Field:   "test-field",
					},
				},
			},
		},
		{
			name: "listing images returns an error response",
			nclient: &mocknclient{
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
					err := resp.SetData(*vmmv4error.NewErrorResponse())
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To("test-image"),
				},
			},
			want: preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "failed to get VM Image: failed to get data returned by ListImages",
						Field:   "test-field",
					},
				},
			},
		},
		{
			name:           "neither image nor imageLookup specified",
			nclient:        &mocknclient{},
			machineDetails: &carenv1.NutanixMachineDetails{
				// both Image and ImageLookup are nil
			},
			want: preflight.CheckResult{
				Allowed: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the check
			check := &imageCheck{
				machineDetails: tc.machineDetails,
				field:          "test-field",
				nclient:        tc.nclient,
			}

			// Execute the check
			got := check.Run(context.Background())

			// Verify the result
			assert.Equal(t, tc.want.Allowed, got.Allowed)
			assert.Equal(t, tc.want.InternalError, got.InternalError)
			assert.Equal(t, tc.want.Causes, got.Causes)
		})
	}
}

func TestGetVMImages(t *testing.T) {
	testCases := []struct {
		name     string
		client   *mocknclient
		id       *capxv1.NutanixResourceIdentifier
		want     []vmmv4.Image
		wantErr  bool
		errorMsg string
	}{
		{
			name: "get image by uuid success",
			client: &mocknclient{
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
			client: &mocknclient{
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
			client: &mocknclient{
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
			client: &mocknclient{
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
			client: &mocknclient{},
			id:     &capxv1.NutanixResourceIdentifier{
				// Both Name and UUID are not set
			},
			wantErr:  true,
			errorMsg: "image identifier is missing both name and uuid",
		},
		{
			name: "no image found by uuid",
			client: &mocknclient{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					return nil, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid"),
			},
			wantErr: false,
			want:    []vmmv4.Image{}, // No images found
		},
		{
			name: "invalid data from GetImageById",
			client: &mocknclient{
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
			client: &mocknclient{
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

func TestNewVMImageChecks(t *testing.T) {
	testCases := []struct {
		name                                      string
		nutanixClusterConfigSpec                  *carenv1.NutanixClusterConfigSpec
		nutanixWorkerNodeConfigSpecByMDName       map[string]*carenv1.NutanixWorkerNodeConfigSpec
		nclient                                   client
		expectedChecks                            int
		expectedControlPlaneCheckFieldIncluded    bool
		expectedWorkerNodeCheckFieldPatternExists bool
	}{
		{
			name:                                   "client not initialized",
			nutanixClusterConfigSpec:               nil,
			nutanixWorkerNodeConfigSpecByMDName:    nil,
			nclient:                                nil,
			expectedChecks:                         0,
			expectedControlPlaneCheckFieldIncluded: false,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name:                                   "no nutanix configuration",
			nutanixClusterConfigSpec:               nil,
			nutanixWorkerNodeConfigSpecByMDName:    nil,
			nclient:                                &mocknclient{},
			expectedChecks:                         0,
			expectedControlPlaneCheckFieldIncluded: false,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name: "control plane configuration only",
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
			nutanixWorkerNodeConfigSpecByMDName: nil,
			nclient: &mocknclient{
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
			expectedChecks:                            1,
			expectedControlPlaneCheckFieldIncluded:    true,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name:                     "worker nodes configuration only",
			nutanixClusterConfigSpec: nil,
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
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
			nclient: &mocknclient{
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
			expectedChecks:                            1,
			expectedControlPlaneCheckFieldIncluded:    false,
			expectedWorkerNodeCheckFieldPatternExists: true,
		},
		{
			name: "both control plane and worker nodes configuration",
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
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Image: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierUUID,
								UUID: ptr.To("test-uuid"),
							},
						},
					},
				},
				"worker-2": {
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
			nclient: &mocknclient{
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
			expectedChecks:                            3, // 1 control plane + 2 workers
			expectedControlPlaneCheckFieldIncluded:    true,
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
			nclient: &mocknclient{
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
			expectedChecks:                            1, // only worker-2
			expectedControlPlaneCheckFieldIncluded:    false,
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
			nclient:                                   &mocknclient{},
			expectedChecks:                            0,
			expectedControlPlaneCheckFieldIncluded:    false,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cd := &checkDependencies{
				nutanixClusterConfigSpec:                           tc.nutanixClusterConfigSpec,
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: tc.nutanixWorkerNodeConfigSpecByMDName,
				nclient: tc.nclient,
				log:     testr.New(t),
			}

			// Call the method under test
			checks := newVMImageChecks(cd)

			// Verify number of checks
			assert.Len(t, checks, tc.expectedChecks)

			results := make([]preflight.CheckResult, len(checks))
			for i, check := range checks {
				results[i] = check.Run(context.Background())
			}
		})
	}
}
