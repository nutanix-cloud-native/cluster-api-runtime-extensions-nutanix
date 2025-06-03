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

// mockV4Client is a mock implementation of the v4client interface for testing
type mockV4Client struct {
	getImageByIdFunc func(uuid *string) (*vmmv4.GetImageApiResponse, error)
	listImagesFunc   func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error)
}

func (m *mockV4Client) GetImageById(uuid *string) (*vmmv4.GetImageApiResponse, error) {
	return m.getImageByIdFunc(uuid)
}

func (m *mockV4Client) ListImages(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
	return m.listImagesFunc(page, limit, filter, orderby, select_)
}

func TestVMImageCheck(t *testing.T) {
	testCases := []struct {
		name           string
		v4client       *mockV4Client
		machineDetails *carenv1.NutanixMachineDetails
		want           preflight.CheckResult
	}{
		{
			name:     "v4client not initialized",
			v4client: nil, // nil client
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
					UUID: ptr.To("test-uuid"),
				},
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "Nutanix v4 client is not initialized, cannot perform VM image checks",
						Field:   "",
					},
				},
			},
		},
		{
			name:     "imageLookup not supported",
			v4client: &mockV4Client{},
			machineDetails: &carenv1.NutanixMachineDetails{
				ImageLookup: &capxv1.NutanixImageLookup{},
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "ImageLookup is not yet supported",
						Field:   "test-field",
					},
				},
			},
		},
		{
			name: "image found by uuid",
			v4client: &mockV4Client{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					resp := &vmmv4.GetImageApiResponse{
						Data: &vmmv4.OneOfGetImageApiResponseData{},
					}
					resp.SetData(&vmmv4.Image{
						ExtId: ptr.To("test-uuid"),
					})
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
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
			v4client: &mockV4Client{
				listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
					resp := &vmmv4.ListImagesApiResponse{}
					resp.SetData([]vmmv4.Image{
						{
							Name: ptr.To("test-image-name"),
						},
					})
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
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
			v4client: &mockV4Client{
				listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
					return &vmmv4.ListImagesApiResponse{}, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
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
			v4client: &mockV4Client{
				listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
					resp := &vmmv4.ListImagesApiResponse{}
					resp.SetData([]vmmv4.Image{
						{
							Name: ptr.To("test-duplicate-image"),
						},
						{
							Name: ptr.To("test-duplicate-image"),
						},
					})
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
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
			v4client: &mockV4Client{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
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
			v4client: &mockV4Client{
				listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Image: &capxv1.NutanixResourceIdentifier{
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
			v4client:       &mockV4Client{},
			machineDetails: &carenv1.NutanixMachineDetails{
				// both Image and ImageLookup are nil
			},
			want: preflight.CheckResult{
				Name:    "NutanixVMImage",
				Allowed: false,
			},
		},
		// {
		// 	name: "invalid response type from GetImageById",
		// 	v4client: &mockV4Client{
		// 		getImageByIdFunc: func(uuid string) (*vmmv4.GetImageApiResponse, error) {
		// 			return &vmmv4.GetImageApiResponse{
		// 				Data: "not-an-image", // Wrong data type
		// 			}, nil
		// 		},
		// 	},
		// 	machineDetails: &carenv1.NutanixMachineDetails{
		// 		Image: &capxv1.NutanixResourceIdentifier{
		// 			UUID: ptr.To("test-uuid"),
		// 		},
		// 	},
		// 	want: preflight.CheckResult{
		// 		Name:    "NutanixVMImage",
		// 		Allowed: false,
		// 		Error:   true,
		// 		Causes: []preflight.Cause{
		// 			{
		// 				Message: "failed to get VM Image: failed to get data returned by GetImageById",
		// 				Field:   "test-field",
		// 			},
		// 		},
		// 	},
		// },
		// {
		// 	name: "invalid response type from ListImages",
		// 	v4client: &mockV4Client{
		// 		listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{},) (*vmmv4.ListImagesApiResponse, error) {
		// 			return &vmmv4.ListImagesApiResponse{
		// 				Data: "not-image-array", // Wrong data type
		// 			}, nil
		// 		},
		// 	},
		// 	machineDetails: &carenv1.NutanixMachineDetails{
		// 		Image: &capxv1.NutanixResourceIdentifier{
		// 			Name: ptr.To("test-image"),
		// 		},
		// 	},
		// 	want: preflight.CheckResult{
		// 		Name:    "NutanixVMImage",
		// 		Allowed: false,
		// 		Error:   true,
		// 		Causes: []preflight.Cause{
		// 			{
		// 				Message: "failed to get VM Image: failed to get data returned by ListImages",
		// 				Field:   "test-field",
		// 			},
		// 		},
		// 	},
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testr.New(t)

			// Create the checker with the mock client
			nc := &nutanixChecker{
				log:      logger,
				v4client: tc.v4client,
			}

			// Call the method under test
			checkFn := nc.vmImageCheck(tc.machineDetails, "test-field")

			// Execute the check
			got := checkFn(context.Background())

			// Verify the result
			assert.Equal(t, tc.want.Name, got.Name)
			assert.Equal(t, tc.want.Allowed, got.Allowed)
			assert.Equal(t, tc.want.Error, got.Error)
			assert.Equal(t, len(tc.want.Causes), len(got.Causes))

			if len(tc.want.Causes) > 0 {
				for i, cause := range tc.want.Causes {
					assert.Equal(t, cause.Message, got.Causes[i].Message)
					assert.Equal(t, cause.Field, got.Causes[i].Field)
				}
			}
		})
	}
}

func TestGetVMImages(t *testing.T) {
	testCases := []struct {
		name     string
		client   *mockV4Client
		id       *capxv1.NutanixResourceIdentifier
		want     []vmmv4.Image
		wantErr  bool
		errorMsg string
	}{
		{
			name: "get image by uuid success",
			client: &mockV4Client{
				getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
					assert.Equal(t, "test-uuid", uuid)
					resp := &vmmv4.GetImageApiResponse{}
					resp.SetData(&vmmv4.Image{
						ExtId: ptr.To("test-uuid"),
					})
					return resp, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid"),
			},
			want: []vmmv4.Image{
				{
					ExtId: ptr.To("test-uuid"),
				},
			},
			wantErr: false,
		},
		{
			name: "get image by name success",
			client: &mockV4Client{
				listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
					assert.NotNil(t, filter)
					assert.Equal(t, "name eq 'test-name'", *filter)
					resp := &vmmv4.ListImagesApiResponse{}
					resp.SetData([]vmmv4.Image{
						{
							Name: ptr.To("test-name"),
						},
					})
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
			client: &mockV4Client{
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
			client: &mockV4Client{
				listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
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
			client: &mockV4Client{},
			id:     &capxv1.NutanixResourceIdentifier{
				// Both Name and UUID are not set
			},
			wantErr:  true,
			errorMsg: "image identifier is missing both name and uuid",
		},
		// {
		// 	name: "invalid data from GetImageById",
		// 	client: &mockV4Client{
		// 		getImageByIdFunc: func(uuid *string) (*vmmv4.GetImageApiResponse, error) {
		// 			return &vmmv4.GetImageApiResponse{
		// 				Data: "not-an-image", // Wrong type
		// 			}, nil
		// 		},
		// 	},
		// 	id: &capxv1.NutanixResourceIdentifier{
		// 		UUID: ptr.To("test-uuid"),
		// 	},
		// 	wantErr:  true,
		// 	errorMsg: "failed to get data returned by GetImageById",
		// },
		// {
		// 	name: "invalid data from ListImages",
		// 	client: &mockV4Client{
		// 		listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
		// 			return &vmmv4.ListImagesApiResponse{
		// 				Data: "not-image-array", // Wrong type
		// 			}, nil
		// 		},
		// 	},
		// 	id: &capxv1.NutanixResourceIdentifier{
		// 		Name: ptr.To("test-name"),
		// 	},
		// 	wantErr:  true,
		// 	errorMsg: "failed to get data returned by ListImages",
		// },
		{
			name: "empty response from ListImages",
			client: &mockV4Client{
				listImagesFunc: func(page *int, limit *int, filter *string, orderby *string, select_ *string, args ...map[string]interface{}) (*vmmv4.ListImagesApiResponse, error) {
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
