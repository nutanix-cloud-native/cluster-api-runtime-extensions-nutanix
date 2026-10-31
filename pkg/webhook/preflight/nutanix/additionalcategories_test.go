// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	prismv4 "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestAdditionalCategoriesCheck(t *testing.T) {
	testCases := []struct {
		name           string
		nclient        client
		machineDetails *carenv1.NutanixMachineDetails
		pcVersion      string
		want           preflight.CheckResult
	}{
		{
			name:           "no additional categories configured",
			nclient:        &clientWrapper{},
			machineDetails: &carenv1.NutanixMachineDetails{},
			pcVersion:      "7.6",
			want:           preflight.CheckResult{Allowed: true},
		},
		{
			name: "additional category found in project scope",
			nclient: &clientWrapper{
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					return nil, projectScopedDefaultProjectErr()
				},
				ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
					return &resolvedProject{UUID: "project-uuid"}, nil
				},
				ListCategoriesByFilterFunc: func(
					ctx context.Context,
					filter string,
				) ([]prismv4.Category, error) {
					assert.Contains(t, filter, "key eq 'env'")
					assert.Contains(t, filter, "value eq 'prod'")
					assert.Contains(t, filter, "projectExtId eq 'project-uuid'")
					return []prismv4.Category{
						{
							Key:   new("env"),
							Value: new("prod"),
						},
					}, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Project: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("project-uuid"),
				},
				AdditionalCategories: []capxv1.NutanixCategoryIdentifier{
					{
						Key:   "env",
						Value: "prod",
					},
				},
			},
			pcVersion: "7.6",
			want:      preflight.CheckResult{Allowed: true},
		},
		{
			name: "additional category not found in project scope",
			nclient: &clientWrapper{
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					return nil, projectScopedDefaultProjectErr()
				},
				ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
					return &resolvedProject{UUID: "project-uuid"}, nil
				},
				ListCategoriesByFilterFunc: func(
					ctx context.Context,
					filter string,
				) ([]prismv4.Category, error) {
					return []prismv4.Category{}, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				Project: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierUUID,
					UUID: new("project-uuid"),
				},
				AdditionalCategories: []capxv1.NutanixCategoryIdentifier{
					{
						Key:   "env",
						Value: "prod",
					},
				},
			},
			pcVersion: "7.6",
			want: preflight.CheckResult{
				Allowed: false,
				Causes: []preflight.Cause{
					{
						Message: "No category \"env\":\"prod\" found in Prism Central for the effective project scope",
						Field:   "test-field.additionalCategories",
					},
				},
			},
		},
		{
			name: "category lookup error",
			nclient: &clientWrapper{
				ListCategoriesByFilterFunc: func(
					ctx context.Context,
					filter string,
				) ([]prismv4.Category, error) {
					return nil, fmt.Errorf("api error")
				},
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					return &PCDefaultProject{
						UUID: "project-uuid",
						Name: "default",
					}, nil
				},
			},
			machineDetails: &carenv1.NutanixMachineDetails{
				AdditionalCategories: []capxv1.NutanixCategoryIdentifier{
					{
						Key:   "env",
						Value: "prod",
					},
				},
			},
			pcVersion: "7.6",
			want: preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "Failed to validate additional category \"env\":\"prod\": api error",
						Field:   "test-field.additionalCategories",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			check := &additionalCategoriesCheck{
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

func TestNewAdditionalCategoriesChecks(t *testing.T) {
	cd := &checkDependencies{
		nclient:   &clientWrapper{},
		pcVersion: "7.6",
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{},
			},
		},
		nutanixWorkerNodeConfigSpecByMachineDeploymentName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
			"md-1": {
				Nutanix: &carenv1.NutanixWorkerNodeSpec{},
			},
		},
	}

	checks := newAdditionalCategoriesChecks(cd)
	require.Len(t, checks, 2)
	assert.Equal(t, "NutanixAdditionalCategories", checks[0].Name())
	assert.Equal(t, "NutanixAdditionalCategories", checks[1].Name())
}

func TestGetCategoriesInProject_UsesProjectFilter(t *testing.T) {
	t.Parallel()

	callCount := 0
	nclient := &clientWrapper{
		ListCategoriesByFilterFunc: func(
			ctx context.Context,
			filter string,
		) ([]prismv4.Category, error) {
			callCount++
			assert.Contains(t, filter, "key eq 'env'")
			assert.Contains(t, filter, "value eq 'prod'")
			if callCount == 1 {
				assert.Contains(t, filter, "projectExtId eq 'project-uuid-1'")
				return []prismv4.Category{}, nil
			}
			assert.Contains(t, filter, "sharedWithProjects/any(a:a eq 'project-uuid-1')")
			assert.Contains(t, filter, "isSharedWithAllProjects eq true")
			return []prismv4.Category{
				{
					Key:   new("env"),
					Value: new("prod"),
				},
			}, nil
		},
	}

	categories, err := getCategoriesInProject(context.Background(), nclient, "env", "prod", "project-uuid-1")
	require.NoError(t, err)
	require.Len(t, categories, 1)
	assert.Equal(t, 2, callCount)
}

func TestGetCategoriesInProject_PrefersProjectOwned(t *testing.T) {
	t.Parallel()

	callCount := 0
	nclient := &clientWrapper{
		ListCategoriesByFilterFunc: func(
			ctx context.Context,
			filter string,
		) ([]prismv4.Category, error) {
			callCount++
			assert.Equal(t, 1, callCount)
			assert.Contains(t, filter, "projectExtId eq 'project-uuid-1'")
			return []prismv4.Category{{Key: new("env"), Value: new("prod")}}, nil
		},
	}

	categories, err := getCategoriesInProject(context.Background(), nclient, "env", "prod", "project-uuid-1")
	require.NoError(t, err)
	require.Len(t, categories, 1)
	assert.Equal(t, 1, callCount)
}
