// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	commonv1 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/common/v1/config"
	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	converged "github.com/nutanix-cloud-native/prism-go-client/converged"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func machineDetailsWithCluster(uuid string) *carenv1.NutanixMachineDetails {
	return &carenv1.NutanixMachineDetails{
		Cluster: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierUUID,
			UUID: new(uuid),
		},
	}
}

func kvPair(t *testing.T, name, value string) commonv1.KVPair {
	t.Helper()
	v := commonv1.NewOneOfKVPairValue()
	require.NoError(t, v.SetValue(value))
	return commonv1.KVPair{Name: new(name), Value: v}
}

func clusterByIDFunc(t *testing.T, version string) func(
	context.Context, *string, ...map[string]any,
) (*clustermgmtv4.GetClusterApiResponse, error) {
	t.Helper()
	return clusterByIDBuildInfoFunc(t, version, "")
}

func clusterByIDBuildInfoFunc(t *testing.T, version, fullVersion string) func(
	context.Context, *string, ...map[string]any,
) (*clustermgmtv4.GetClusterApiResponse, error) {
	t.Helper()
	return func(_ context.Context, uuid *string, _ ...map[string]any) (*clustermgmtv4.GetClusterApiResponse, error) {
		cluster := clustermgmtv4.Cluster{
			ObjectType_: new("clustermgmt.v4.config.Cluster"),
			ExtId:       uuid,
			Name:        new("pe-cluster"),
		}
		if version != "" || fullVersion != "" {
			buildInfo := &clustermgmtv4.BuildReference{}
			if version != "" {
				buildInfo.Version = new(version)
			}
			if fullVersion != "" {
				buildInfo.FullVersion = new(fullVersion)
			}
			cluster.Config = &clustermgmtv4.ClusterConfigReference{BuildInfo: buildInfo}
		}
		resp := &clustermgmtv4.GetClusterApiResponse{
			ObjectType_: new("clustermgmt.v4.config.GetClusterApiResponse"),
		}
		require.NoError(t, resp.SetData(cluster))
		return resp, nil
	}
}

func TestAOSVersionFromCluster(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", aosVersionFromCluster(nil))
	assert.Equal(t, "", aosVersionFromCluster(&clustermgmtv4.Cluster{}))
	assert.Equal(t, "", aosVersionFromCluster(&clustermgmtv4.Cluster{
		Config: &clustermgmtv4.ClusterConfigReference{},
	}))
	assert.Equal(t, "7.6", aosVersionFromCluster(&clustermgmtv4.Cluster{
		Config: &clustermgmtv4.ClusterConfigReference{
			BuildInfo: &clustermgmtv4.BuildReference{Version: new(" 7.6 ")},
		},
	}))
	assert.Equal(t, "7.5.1", aosVersionFromCluster(&clustermgmtv4.Cluster{
		Config: &clustermgmtv4.ClusterConfigReference{
			BuildInfo: &clustermgmtv4.BuildReference{
				Version:     new("7.5"),
				FullVersion: new("el8.5-release-ganges-7.5.1-stable-b2a0591e15d312a5888e2eee14e1c4bffc3e3b3d"),
			},
		},
	}))
	assert.Equal(t, "7.5.1", aosVersionFromCluster(&clustermgmtv4.Cluster{
		Config: &clustermgmtv4.ClusterConfigReference{
			BuildInfo: &clustermgmtv4.BuildReference{
				FullVersion: new("el8.5-release-ganges-7.5.1-stable-b2a0591e15d312a5888e2eee14e1c4bffc3e3b3d"),
			},
		},
	}))
}

func TestPlacementTargetAOSVersion(t *testing.T) {
	t.Parallel()

	target := &multidomainv4.TargetDetails{
		ClusterExtId: new("pe-uuid"),
		Capabilities: []commonv1.KVPair{
			kvPair(t, "cluster_name", "pe-name"),
			kvPair(t, "full_version", "el9-release-ganges-7.6-stable-abc123"),
			kvPair(t, "version", "7.6"),
		},
	}
	assert.Equal(t, "7.6", placementTargetAOSVersion(target))

	// Short version is truncated; full_version carries the patch.
	targetTruncated := &multidomainv4.TargetDetails{
		Capabilities: []commonv1.KVPair{
			kvPair(t, "full_version", "el8.5-release-ganges-7.5.1-stable-b2a0591e15d312a5888e2eee14e1c4bffc3e3b3d"),
			kvPair(t, "version", "7.5"),
		},
	}
	assert.Equal(t, "7.5.1", placementTargetAOSVersion(targetTruncated))

	// Falls back to the AOS token in full_version when "version" is absent.
	targetNoVersion := &multidomainv4.TargetDetails{
		Capabilities: []commonv1.KVPair{
			kvPair(t, "full_version", "el9-release-ganges-7.6-stable-abc123"),
		},
	}
	assert.Equal(t, "7.6", placementTargetAOSVersion(targetNoVersion))
}

func TestPlacementTargetMatchesCluster(t *testing.T) {
	t.Parallel()

	target := &multidomainv4.TargetDetails{
		ClusterExtId: new("pe-uuid"),
		Capabilities: []commonv1.KVPair{kvPair(t, "cluster_name", "pe-name")},
	}

	assert.True(t, placementTargetMatchesCluster(target, &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("pe-uuid"),
	}))
	assert.True(t, placementTargetMatchesCluster(target, &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("pe-name"),
	}))
	assert.False(t, placementTargetMatchesCluster(target, &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("other-uuid"),
	}))
	assert.False(t, placementTargetMatchesCluster(target, &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("other-name"),
	}))
}

func TestResolvePrismElementAOSVersion_NonProjectScoped(t *testing.T) {
	t.Parallel()

	clusterID := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("pe-uuid"),
	}
	nclient := &clientWrapper{
		GetClusterByIdFunc: clusterByIDFunc(t, "7.6"),
	}

	// PC 7.5 is below the project-policy threshold, so no project-scope resolution happens and the
	// cluster-management API is used to read the AOS version.
	version, err := resolvePrismElementAOSVersion(context.Background(), nclient, "7.5", clusterID, nil)
	require.NoError(t, err)
	assert.Equal(t, "7.6", version)
}

func TestResolvePrismElementAOSVersion_ProjectScoped(t *testing.T) {
	t.Parallel()

	clusterID := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("pe-uuid"),
	}
	projectRef := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("project-uuid"),
	}
	nclient := &clientWrapper{
		// Project-scoped users cannot read the default project.
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			return nil, projectScopedDefaultProjectErr()
		},
		ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "project-uuid"}, nil
		},
		ListResourceGroupsFunc: func(
			context.Context, ...converged.ODataOption,
		) ([]multidomainv4.ResourceGroup, error) {
			return []multidomainv4.ResourceGroup{
				{
					ExtId: new("rg-1"),
					PlacementTargets: []multidomainv4.TargetDetails{
						{
							ClusterExtId: new("pe-uuid"),
							Capabilities: []commonv1.KVPair{
								kvPair(t, "cluster_name", "pe-name"),
								kvPair(t, "version", "7.6"),
							},
						},
					},
				},
			}, nil
		},
	}

	version, err := resolvePrismElementAOSVersion(context.Background(), nclient, "7.6", clusterID, projectRef)
	require.NoError(t, err)
	assert.Equal(t, "7.6", version)
}

func TestResolvePrismElementAOSVersion_ProjectScoped_NotFound(t *testing.T) {
	t.Parallel()

	clusterID := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("pe-uuid"),
	}
	projectRef := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("project-uuid"),
	}
	nclient := &clientWrapper{
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			return nil, projectScopedDefaultProjectErr()
		},
		ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "project-uuid"}, nil
		},
		ListResourceGroupsFunc: func(
			context.Context, ...converged.ODataOption,
		) ([]multidomainv4.ResourceGroup, error) {
			return []multidomainv4.ResourceGroup{
				{
					ExtId: new("rg-1"),
					PlacementTargets: []multidomainv4.TargetDetails{
						{
							ClusterExtId: new("some-other-pe"),
							Capabilities: []commonv1.KVPair{kvPair(t, "version", "7.6")},
						},
					},
				},
			}, nil
		},
	}

	_, err := resolvePrismElementAOSVersion(context.Background(), nclient, "7.6", clusterID, projectRef)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not determine the AOS version")
}

func TestAOSVersionCheck_Run(t *testing.T) {
	t.Parallel()

	field := "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.machineDetails.cluster"

	tests := []struct {
		name          string
		version       string
		minVersion    string
		expectAllowed bool
		expectCause   string
	}{
		{
			name:          "equal to floor passes",
			version:       "7.5",
			minVersion:    minSupportedAOSVersion,
			expectAllowed: true,
		},
		{
			name:          "above floor passes",
			version:       "7.6",
			minVersion:    minSupportedAOSVersion,
			expectAllowed: true,
		},
		{
			name:          "below floor fails",
			version:       "7.4",
			minVersion:    minSupportedAOSVersion,
			expectAllowed: false,
			expectCause:   "older than the minimum supported version",
		},
		{
			name:          "internal build passes",
			version:       "master",
			minVersion:    minSupportedAOSVersion,
			expectAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := &aosVersionCheck{
				machineSpec: machineDetailsWithCluster("pe-uuid"),
				field:       field,
				nclient: &clientWrapper{
					GetClusterByIdFunc: clusterByIDFunc(t, tt.version),
				},
				minVersion: tt.minVersion,
				pcVersion:  "7.5",
			}

			result := check.Run(context.Background())
			assert.Equal(t, tt.expectAllowed, result.Allowed)
			if tt.expectCause != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tt.expectCause)
			}
		})
	}
}

func TestAOSVersionCheck_Run_NoClusterReferenceIsAllowed(t *testing.T) {
	t.Parallel()

	check := &aosVersionCheck{
		machineSpec: &carenv1.NutanixMachineDetails{},
		field:       "field",
		nclient:     &clientWrapper{},
		minVersion:  minSupportedAOSVersion,
		pcVersion:   "7.5",
	}
	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
}

func TestAOSVersionCheck_Run_ErrMessageFailsClosed(t *testing.T) {
	t.Parallel()

	check := &aosVersionCheck{
		field:      "field",
		minVersion: minSupportedAOSVersion,
		errMessage: new("boom"),
	}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	require.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "boom")
}

func TestIdentifierKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", identifierKey(nil))
	assert.Equal(t, "uuid:abc", identifierKey(&capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("ABC"),
	}))
	assert.Equal(t, "name:pe", identifierKey(&capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("PE"),
	}))
}
