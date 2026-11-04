// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	vmmv4ahv "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestVMProfileCheck(t *testing.T) {
	testCases := []struct {
		name           string
		nclient        client
		machineDetails *carenv1.NutanixMachineDetails
		pcVersion      string
		want           preflight.CheckResult
	}{
		{
			name:           "vmProfile not set - should pass",
			nclient:        &clientWrapper{},
			machineDetails: &carenv1.NutanixMachineDetails{},
			want: preflight.CheckResult{
				Allowed: true,
			},
			pcVersion: "7.3",
		},
		{
			name: "vmProfile found by uuid",
			nclient: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					assert.Equal(t, "test-uuid", *uuid)
					resp := &vmmv4ahv.GetVmProfileApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfGetVmProfileApiResponseData()
					err := resp.Data.SetValue(vmmv4ahv.VmProfile{
						ObjectType_: new("vmm.v4.ahv.config.VmProfile"),
						ExtId:       new("test-uuid"),
						VmConfiguration: &vmmv4ahv.VmProfileVmConfiguration{
							Nics: []vmmv4ahv.VmProfileNic{
								{
									ExtId: new("nic-ext-id-1"),
								},
							},
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("test-uuid"),
				},
			},
			want: preflight.CheckResult{
				Allowed: true,
			},
			pcVersion: "7.3",
		},
		{
			name: "vmProfile not found by uuid",
			nclient: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					return nil, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("test-uuid"),
				},
			},
			want: preflight.CheckResult{
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: "No VM Profile found in Prism Central that matches identifier",
						Field:   "test-field.vmProfile",
					},
				},
			},
			pcVersion: "7.3",
		},
		{
			name: "vmProfile found by name",
			nclient: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					assert.NotNil(t, filter_)
					assert.Contains(t, *filter_, "name eq 'test-profile-name'")
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{
						{
							Name: new("test-profile-name"),
							VmConfiguration: &vmmv4ahv.VmProfileVmConfiguration{
								Nics: []vmmv4ahv.VmProfileNic{
									{
										ExtId: new("nic-ext-id-1"),
									},
								},
							},
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("test-profile-name"),
				},
			},
			want: preflight.CheckResult{
				Allowed: true,
			},
			pcVersion: "7.3",
		},
		{
			name: "vmProfile found by name in project scope",
			nclient: &clientWrapper{
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					return nil, projectScopedDefaultProjectErr()
				},
				ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
					return &resolvedProject{UUID: "project-uuid"}, nil
				},
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					assert.NotNil(t, filter_)
					assert.Contains(t, *filter_, "name eq 'test-profile-name'")
					assert.Contains(t, *filter_, "projectExtId eq 'project-uuid'")
					assert.Contains(t, *filter_, "sharedWithProjects/any(a:a eq 'project-uuid')")
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{
						{
							Name: new("test-profile-name"),
							VmConfiguration: &vmmv4ahv.VmProfileVmConfiguration{
								Nics: []vmmv4ahv.VmProfileNic{
									{
										ExtId: new("nic-ext-id-1"),
									},
								},
							},
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("test-profile-name"),
				},
				Project: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("project-uuid"),
				},
			},
			want: preflight.CheckResult{
				Allowed: true,
			},
			pcVersion: "7.6",
		},
		{
			name: "vmProfile not found by name",
			nclient: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("non-existent-profile"),
				},
			},
			want: preflight.CheckResult{
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: "No VM Profile found in Prism Central that matches identifier",
						Field:   "test-field.vmProfile",
					},
				},
			},
			pcVersion: "7.3",
		},
		{
			name: "multiple vmProfiles found by name",
			nclient: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{
						{Name: new("duplicate-profile")},
						{Name: new("duplicate-profile")},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("duplicate-profile"),
				},
			},
			want: preflight.CheckResult{
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: "Found 2 VM Profiles in Prism Central that match name",
						Field:   "test-field.vmProfile",
					},
				},
			},
			pcVersion: "7.3",
		},
		{
			name: "error getting vmProfile by id",
			nclient: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("test-uuid"),
				},
			},
			want: preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "Failed to get VM Profile",
						Field:   "test-field.vmProfile",
					},
				},
			},
			pcVersion: "7.3",
		},
		{
			name: "error listing vmProfiles",
			nclient: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("test-profile"),
				},
			},
			want: preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "Failed to get VM Profile",
						Field:   "test-field.vmProfile",
					},
				},
			},
			pcVersion: "7.3",
		},
		{
			name: "vmProfile found by uuid without NIC profiles configured",
			nclient: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					assert.Equal(t, "test-uuid", *uuid)
					resp := &vmmv4ahv.GetVmProfileApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfGetVmProfileApiResponseData()
					err := resp.Data.SetValue(vmmv4ahv.VmProfile{
						ObjectType_: new("vmm.v4.ahv.config.VmProfile"),
						ExtId:       new("test-uuid"),
						// VmConfiguration is nil or has no Nics
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("test-uuid"),
				},
			},
			want: preflight.CheckResult{
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: "The VMProfile chosen",
						Field:   "test-field.vmProfile",
					},
				},
			},
			pcVersion: "7.3",
		},
		{
			name: "vmProfile found by name without NIC profiles configured",
			nclient: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					assert.NotNil(t, filter_)
					assert.Contains(t, *filter_, "name eq 'test-profile-name'")
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{
						{
							Name: new("test-profile-name"),
							// VmConfiguration is nil or has no Nics
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				VMProfile: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: new("test-profile-name"),
				},
			},
			want: preflight.CheckResult{
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: "The VMProfile chosen",
						Field:   "test-field.vmProfile",
					},
				},
			},
			pcVersion: "7.3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			check := &vmProfileCheck{
				machineDetails: tc.machineDetails,
				field:          "test-field",
				nclient:        tc.nclient,
				pcVersion:      tc.pcVersion,
				projectRef:     tc.machineDetails.Project,
			}

			got := check.Run(context.Background())

			assert.Equal(t, tc.want.Allowed, got.Allowed)
			assert.Equal(t, tc.want.InternalError, got.InternalError)
			if len(tc.want.Causes) > 0 {
				require.NotEmpty(t, got.Causes)
				assert.Contains(t, got.Causes[0].Message, tc.want.Causes[0].Message)
				assert.Equal(t, tc.want.Causes[0].Field, got.Causes[0].Field)
			}
		})
	}
}

func TestGetVMProfilesInProject(t *testing.T) {
	testCases := []struct {
		name        string
		client      *clientWrapper
		id          *capxv1.NutanixResourceIdentifier
		projectUUID string
		wantErr     bool
		errorMsg    string
	}{
		{
			name: "get vmProfile by name in project success",
			client: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					assert.NotNil(t, filter_)
					assert.Contains(t, *filter_, "name eq 'test-name'")
					assert.Contains(t, *filter_, "projectExtId eq 'project-uuid'")
					assert.Contains(t, *filter_, "sharedWithProjects/any(a:a eq 'project-uuid')")
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{{Name: new("test-name")}})
					require.NoError(t, err)
					return resp, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: new("test-name"),
			},
			projectUUID: "project-uuid",
		},
		{
			name: "get vmProfile by uuid in project success",
			client: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					assert.NotNil(t, filter_)
					assert.Contains(t, *filter_, "extId eq 'test-uuid'")
					assert.Contains(t, *filter_, "projectExtId eq 'project-uuid'")
					assert.Contains(t, *filter_, "sharedWithProjects/any(a:a eq 'project-uuid')")
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{{ExtId: new("test-uuid")}})
					require.NoError(t, err)
					return resp, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("test-uuid"),
			},
			projectUUID: "project-uuid",
		},
		{
			name:        "invalid identifier",
			client:      &clientWrapper{},
			id:          &capxv1.NutanixResourceIdentifier{},
			wantErr:     true,
			errorMsg:    "vm profile identifier is missing both name and uuid",
			projectUUID: "project-uuid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := getVMProfilesInProject(context.Background(), tc.client, tc.id, tc.projectUUID)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetVMProfiles(t *testing.T) {
	testCases := []struct {
		name     string
		client   *clientWrapper
		id       *capxv1.NutanixResourceIdentifier
		want     []vmmv4ahv.VmProfile
		wantErr  bool
		errorMsg string
	}{
		{
			name: "get vmProfile by uuid success",
			client: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					assert.Equal(t, "test-uuid", *uuid)
					resp := &vmmv4ahv.GetVmProfileApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfGetVmProfileApiResponseData()
					err := resp.Data.SetValue(vmmv4ahv.VmProfile{
						ObjectType_: new("vmm.v4.ahv.config.VmProfile"),
						ExtId:       new("test-uuid"),
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("test-uuid"),
			},
			want: []vmmv4ahv.VmProfile{
				{
					ObjectType_: new("vmm.v4.ahv.config.VmProfile"),
					ExtId:       new("test-uuid"),
				},
			},
			wantErr: false,
		},
		{
			name: "get vmProfile by name success",
			client: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					assert.NotNil(t, filter_)
					assert.Equal(t, "name eq 'test-name'", *filter_)
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{
						{Name: new("test-name")},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: new("test-name"),
			},
			want: []vmmv4ahv.VmProfile{
				{Name: new("test-name")},
			},
			wantErr: false,
		},
		{
			name: "get vmProfile by uuid error",
			client: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("test-uuid"),
			},
			wantErr:  true,
			errorMsg: "api error",
		},
		{
			name: "get vmProfile by name error",
			client: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					return nil, fmt.Errorf("api error")
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: new("test-name"),
			},
			wantErr:  true,
			errorMsg: "api error",
		},
		{
			name:     "neither name nor uuid specified",
			client:   &clientWrapper{},
			id:       &capxv1.NutanixResourceIdentifier{},
			wantErr:  true,
			errorMsg: "vm profile identifier is missing both name and uuid",
		},
		{
			name: "no vmProfile found by uuid",
			client: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					return nil, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("test-uuid"),
			},
			wantErr: false,
			want:    []vmmv4ahv.VmProfile{},
		},
		{
			name: "no vmProfile found by name",
			client: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					resp := &vmmv4ahv.ListVmProfilesApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
					err := resp.Data.SetValue([]vmmv4ahv.VmProfile{})
					require.NoError(t, err)
					return resp, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: new("test-name"),
			},
			wantErr: false,
			want:    []vmmv4ahv.VmProfile{},
		},
		{
			name: "invalid data from GetVMProfileById",
			client: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					return &vmmv4ahv.GetVmProfileApiResponse{
						Data: nil,
					}, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("test-uuid"),
			},
			wantErr:  true,
			errorMsg: "failed to get data returned by GetVMProfileById",
		},
		{
			name: "empty response from ListVMProfiles",
			client: &clientWrapper{
				ListVMProfilesFunc: func(ctx context.Context, page_, limit_ *int, filter_, orderby_, select_ *string, args ...map[string]any) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
					return &vmmv4ahv.ListVmProfilesApiResponse{
						Data: nil,
					}, nil
				},
			},
			id: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: new("test-name"),
			},
			want:    []vmmv4ahv.VmProfile{},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			got, err := getVMProfiles(ctx, tc.client, tc.id)

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

func TestNewVMProfileChecks(t *testing.T) {
	testCases := []struct {
		name                                      string
		nutanixClusterConfigSpec                  *carenv1.NutanixClusterConfigSpec
		nutanixWorkerNodeConfigSpecByMDName       map[string]*carenv1.NutanixWorkerNodeConfigSpec
		nclient                                   client
		expectedChecks                            int
		expectedWorkerNodeCheckFieldPatternExists bool
	}{
		{
			name:                                "client not initialized",
			nutanixClusterConfigSpec:            nil,
			nutanixWorkerNodeConfigSpecByMDName: nil,
			nclient:                             nil,
			expectedChecks:                      0,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name:                                "no nutanix configuration",
			nutanixClusterConfigSpec:            nil,
			nutanixWorkerNodeConfigSpecByMDName: nil,
			nclient:                             &clientWrapper{},
			expectedChecks:                      0,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name:                     "worker nodes configuration only",
			nutanixClusterConfigSpec: nil,
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							VMProfile: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierUUID,
								UUID: new("test-uuid"),
							},
						},
					},
				},
			},
			nclient: &clientWrapper{
				GetVMProfileByIdFunc: func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
					resp := &vmmv4ahv.GetVmProfileApiResponse{}
					resp.Data = vmmv4ahv.NewOneOfGetVmProfileApiResponseData()
					err := resp.Data.SetValue(vmmv4ahv.VmProfile{
						ObjectType_: new("vmm.v4.ahv.config.VmProfile"),
						ExtId:       new("test-uuid"),
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectedChecks: 1,
			expectedWorkerNodeCheckFieldPatternExists: true,
		},
		{
			name:                     "multiple worker nodes",
			nutanixClusterConfigSpec: nil,
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							VMProfile: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierUUID,
								UUID: new("test-uuid"),
							},
						},
					},
				},
				"worker-2": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							VMProfile: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierUUID,
								UUID: new("test-uuid-2"),
							},
						},
					},
				},
			},
			nclient:        &clientWrapper{},
			expectedChecks: 2,
			expectedWorkerNodeCheckFieldPatternExists: true,
		},
		{
			name:                     "worker with nil Nutanix config",
			nutanixClusterConfigSpec: nil,
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: nil,
				},
			},
			nclient:        &clientWrapper{},
			expectedChecks: 0,
			expectedWorkerNodeCheckFieldPatternExists: false,
		},
		{
			name:                     "worker nodes without vmProfile",
			nutanixClusterConfigSpec: nil,
			nutanixWorkerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							// VMProfile is nil
						},
					},
				},
			},
			nclient:        &clientWrapper{},
			expectedChecks: 1, // Check is still created, but will pass since VMProfile is nil
			expectedWorkerNodeCheckFieldPatternExists: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cd := &checkDependencies{
				nutanixClusterConfigSpec:                           tc.nutanixClusterConfigSpec,
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: tc.nutanixWorkerNodeConfigSpecByMDName,
				nclient:   tc.nclient,
				pcVersion: "7.3.0",
				log:       testr.New(t),
			}

			checks := newVMProfileChecks(cd)

			assert.Len(t, checks, tc.expectedChecks)

			if tc.expectedWorkerNodeCheckFieldPatternExists && len(checks) > 0 {
				check := checks[0].(*vmProfileCheck)
				assert.Contains(t, check.field, "machineDeployments")
			}
		})
	}
}
