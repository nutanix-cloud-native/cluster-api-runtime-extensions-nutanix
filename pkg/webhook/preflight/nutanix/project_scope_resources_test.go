// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	converged "github.com/nutanix-cloud-native/prism-go-client/converged"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
)

// projectScopedDefaultProjectErr returns the error a project-scoped (Projects 2.0) user gets when
// probing the Prism Central default project: the converged SDK maps HTTP 403 - and the RBAC-hidden
// default-project case (HTTP 200 + empty list) - to ErrUnauthorized. Tests use it to simulate the
// project-scoped path via ResolvePCDefaultProject.
func projectScopedDefaultProjectErr() error {
	return &converged.APIError{Kind: converged.ErrUnauthorized}
}

func TestResolveProjectScopeContext_ProjectScoped(t *testing.T) {
	t.Parallel()

	projectRef := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("00000000-0000-0000-0000-000000000111"),
	}
	nclient := &clientWrapper{
		ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "00000000-0000-0000-0000-000000000111"}, nil
		},
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			return nil, projectScopedDefaultProjectErr()
		},
	}

	scopeCtx, err := resolveProjectScopeContext(context.Background(), nclient, "7.6", projectRef)
	require.NoError(t, err)
	require.NotNil(t, scopeCtx)
	assert.Equal(t, userScopeProjectScoped, scopeCtx.scope)
	assert.Equal(t, "00000000-0000-0000-0000-000000000111", scopeCtx.projectUUID)
}

func TestResolveProjectScopeContext_PassesPCVersionToResolve(t *testing.T) {
	t.Parallel()

	var got string
	nclient := &clientWrapper{
		ResolvePCDefaultProjectFunc: func(_ context.Context, pcVersion string) (*PCDefaultProject, error) {
			got = pcVersion
			return &PCDefaultProject{UUID: "def-uuid", Name: "default"}, nil
		},
	}

	scopeCtx, err := resolveProjectScopeContext(context.Background(), nclient, "7.6.1", nil)
	require.NoError(t, err)
	require.NotNil(t, scopeCtx)
	assert.Equal(t, "7.6.1", got)
	assert.Equal(t, "def-uuid", scopeCtx.projectUUID)
}

func TestResolveProjectScopeContext_FailClosedWhenClientMissing(t *testing.T) {
	t.Parallel()

	scopeCtx, err := resolveProjectScopeContext(context.Background(), nil, "7.6", nil)
	require.Error(t, err)
	assert.Nil(t, scopeCtx)
	assert.Contains(t, err.Error(), "client is not initialized")
}

func TestResolveProjectScopeContext_UsesDefaultProjectWhenRefMissing(t *testing.T) {
	t.Parallel()

	nclient := &clientWrapper{
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			return &PCDefaultProject{
				UUID: "00000000-0000-0000-0000-000000000333",
				Name: "default",
			}, nil
		},
	}

	scopeCtx, err := resolveProjectScopeContext(context.Background(), nclient, "7.6", nil)
	require.NoError(t, err)
	require.NotNil(t, scopeCtx)
	assert.Equal(t, userScopeNonProjectScoped, scopeCtx.scope)
	assert.Equal(t, "00000000-0000-0000-0000-000000000333", scopeCtx.projectUUID)
}

func TestResolveProjectScopeContext_DeniesOnUnauthorizedDefaultProjectLookup(t *testing.T) {
	t.Parallel()

	nclient := &clientWrapper{
		// A genuine 401 (bad/missing credentials) maps to ErrUnauthenticated in the converged SDK
		// and must fail closed rather than be treated as a project-scoped signal.
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			return nil, &converged.APIError{
				Kind:    converged.ErrUnauthenticated,
				Message: "invalid credentials",
			}
		},
	}

	scopeCtx, err := resolveProjectScopeContext(context.Background(), nclient, "7.6", nil)
	require.Error(t, err)
	assert.Nil(t, scopeCtx)
	assert.Contains(t, err.Error(), "unauthenticated")
}

func TestResolveProjectScopeContext_ProjectScopedWhenDefaultProjectUnauthorized(t *testing.T) {
	t.Parallel()

	projectRef := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("coke"),
	}
	nclient := &clientWrapper{
		// prism-go-client >= v0.8.2 GetDefaultProject returns ErrUnauthorized when
		// RBAC hides the system default project (HTTP 200 + empty list) from a project-scoped
		// user. This must be read as "project-scoped", not as a hard failure.
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			return nil, &converged.APIError{
				Kind:    converged.ErrUnauthorized,
				Message: "credentials are not authorized to access the default project",
			}
		},
		ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "00000000-0000-0000-0000-000000000222", Name: "coke"}, nil
		},
	}

	scopeCtx, err := resolveProjectScopeContext(context.Background(), nclient, "7.6", projectRef)
	require.NoError(t, err)
	require.NotNil(t, scopeCtx)
	assert.Equal(t, userScopeProjectScoped, scopeCtx.scope)
	assert.Equal(t, "00000000-0000-0000-0000-000000000222", scopeCtx.projectUUID)
}

func TestIsProjectScopeAccessError(t *testing.T) {
	t.Parallel()

	// HTTP 403 / RBAC-hidden default project (converged maps both to ErrUnauthorized):
	// authenticated but not permitted -> project-scoped signal.
	assert.True(t, isProjectScopeAccessError(&converged.APIError{Kind: converged.ErrUnauthorized}))
	// errors.Is unwraps fmt wrapping, so a wrapped ErrUnauthorized is still detected.
	assert.True(t, isProjectScopeAccessError(
		fmt.Errorf("failed to list all projects: %w", &converged.APIError{Kind: converged.ErrUnauthorized}),
	))

	// HTTP 401 / bad credentials: authentication failure -> not a scoping signal.
	assert.False(t, isProjectScopeAccessError(&converged.APIError{Kind: converged.ErrUnauthenticated}))
	// Any other / unclassified error is not a scoping signal either.
	assert.False(t, isProjectScopeAccessError(fmt.Errorf("some unrelated error")))
}

func TestProjectScopeAllowsCluster_MultiRG(t *testing.T) {
	t.Parallel()

	clusterID := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("cluster-uuid-2"),
	}
	nclient := &clientWrapper{
		ListResourceGroupsFunc: func(
			context.Context,
			...converged.ODataOption,
		) ([]multidomainv4.ResourceGroup, error) {
			return []multidomainv4.ResourceGroup{
				{ExtId: new("rg-1"), Name: new("rg-1")},
				{ExtId: new("rg-2"), Name: new("rg-2")},
			}, nil
		},
		ListPrismElementsByResourceGroupFunc: func(_ context.Context, rgUUID string) ([]converged.PrismElementInfo, error) {
			switch rgUUID {
			case "rg-1":
				return []converged.PrismElementInfo{{ExtId: "cluster-uuid-1", Name: "cluster-a"}}, nil
			case "rg-2":
				return []converged.PrismElementInfo{{ExtId: "cluster-uuid-2", Name: "cluster-b"}}, nil
			default:
				return nil, nil
			}
		},
	}

	allowed, err := projectScopeAllowsCluster(context.Background(), nclient, "project-uuid", clusterID)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestProjectScopeAllowsStorageContainerInSameResourceGroup(t *testing.T) {
	t.Parallel()

	clusterID := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("cluster-uuid-1"),
	}
	nclient := &clientWrapper{
		ListResourceGroupsFunc: func(
			context.Context,
			...converged.ODataOption,
		) ([]multidomainv4.ResourceGroup, error) {
			return []multidomainv4.ResourceGroup{
				{ExtId: new("rg-a"), Name: new("rg-a")},
				{ExtId: new("rg-b"), Name: new("rg-b")},
			}, nil
		},
		ListPrismElementsByResourceGroupFunc: func(_ context.Context, rgUUID string) ([]converged.PrismElementInfo, error) {
			switch rgUUID {
			case "rg-a":
				return []converged.PrismElementInfo{{ExtId: "cluster-uuid-1", Name: "cluster-a"}}, nil
			case "rg-b":
				return []converged.PrismElementInfo{{ExtId: "cluster-uuid-2", Name: "cluster-b"}}, nil
			default:
				return nil, nil
			}
		},
		ListStorageContainersByResourceGroupFunc: func(_ context.Context, rgUUID string) ([]converged.StorageContainerInfo, error) {
			switch rgUUID {
			case "rg-a":
				return []converged.StorageContainerInfo{
					{
						Name: "sc-a",
						PrismElement: converged.PrismElementInfo{
							ExtId: "cluster-uuid-1",
							Name:  "cluster-a",
						},
					},
				}, nil
			case "rg-b":
				return []converged.StorageContainerInfo{
					{
						Name: "sc-a",
						PrismElement: converged.PrismElementInfo{
							ExtId: "cluster-uuid-2",
							Name:  "cluster-b",
						},
					},
				}, nil
			default:
				return nil, nil
			}
		},
	}

	allowed, err := projectScopeAllowsStorageContainerInSameResourceGroup(
		context.Background(),
		nclient,
		"project-uuid",
		clusterID,
		"sc-a",
	)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestProjectScopeAllowsStorageContainerInSameResourceGroup_DeniesSplitAcrossRGs(t *testing.T) {
	t.Parallel()

	clusterID := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("cluster-uuid-1"),
	}
	nclient := &clientWrapper{
		ListResourceGroupsFunc: func(
			context.Context,
			...converged.ODataOption,
		) ([]multidomainv4.ResourceGroup, error) {
			return []multidomainv4.ResourceGroup{
				{ExtId: new("rg-a"), Name: new("rg-a")},
				{ExtId: new("rg-b"), Name: new("rg-b")},
			}, nil
		},
		ListPrismElementsByResourceGroupFunc: func(_ context.Context, rgUUID string) ([]converged.PrismElementInfo, error) {
			switch rgUUID {
			case "rg-a":
				return []converged.PrismElementInfo{{ExtId: "cluster-uuid-1", Name: "cluster-a"}}, nil
			case "rg-b":
				return []converged.PrismElementInfo{{ExtId: "cluster-uuid-2", Name: "cluster-b"}}, nil
			default:
				return nil, nil
			}
		},
		ListStorageContainersByResourceGroupFunc: func(_ context.Context, rgUUID string) ([]converged.StorageContainerInfo, error) {
			switch rgUUID {
			case "rg-a":
				return []converged.StorageContainerInfo{}, nil
			case "rg-b":
				return []converged.StorageContainerInfo{
					{
						Name: "sc-a",
						PrismElement: converged.PrismElementInfo{
							ExtId: "cluster-uuid-2",
							Name:  "cluster-b",
						},
					},
				}, nil
			default:
				return nil, nil
			}
		},
	}

	allowed, err := projectScopeAllowsStorageContainerInSameResourceGroup(
		context.Background(),
		nclient,
		"project-uuid",
		clusterID,
		"sc-a",
	)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestResourceGroupAllowsStorageContainer(t *testing.T) {
	t.Parallel()
	containers := []converged.StorageContainerInfo{
		{
			Name: "sc-a",
			PrismElement: converged.PrismElementInfo{
				ExtId: "cluster-uuid-1",
				Name:  "cluster-a",
			},
		},
	}
	clusterID := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("cluster-a"),
	}
	assert.True(t, resourceGroupAllowsStorageContainer(containers, clusterID, "sc-a"))
	assert.False(t, resourceGroupAllowsStorageContainer(containers, clusterID, "sc-b"))
}

func TestResourceGroupAllowsClusterAndStorageContainer(t *testing.T) {
	t.Parallel()

	clusterID := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("cluster-uuid-1"),
	}
	clusters := []converged.PrismElementInfo{
		{ExtId: "cluster-uuid-1", Name: "cluster-a"},
	}
	containers := []converged.StorageContainerInfo{
		{
			Name: "sc-a",
			PrismElement: converged.PrismElementInfo{
				ExtId: "cluster-uuid-1",
				Name:  "cluster-a",
			},
		},
	}

	assert.True(t, resourceGroupAllowsClusterAndStorageContainer(clusters, containers, clusterID, "sc-a"))
	assert.False(t, resourceGroupAllowsClusterAndStorageContainer(clusters, containers, clusterID, "sc-b"))
}
