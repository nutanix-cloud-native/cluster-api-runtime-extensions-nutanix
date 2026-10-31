// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	commonv1 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/common/v1/config"
	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	converged "github.com/nutanix-cloud-native/prism-go-client/converged"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
)

// projectScopedClient returns a client that resolves as project-scoped and reports the given AOS
// version for the target Prism Element via the project's resource groups.
func projectScopedClient(t *testing.T, peUUID, aosVersion string) *clientWrapper {
	t.Helper()
	return &clientWrapper{
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
							ClusterExtId: new(peUUID),
							Capabilities: []commonv1.KVPair{
								kvPair(t, "cluster_name", "pe-name"),
								kvPair(t, "version", aosVersion),
							},
						},
					},
				},
			}, nil
		},
	}
}

func projectScopedAOSCheckDeps(storagePolicy string) *checkDependencies {
	return &checkDependencies{
		nclient:   &clientWrapper{},
		pcVersion: "7.6",
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					MachineDetails: *machineDetailsWithCluster("pe-uuid"),
				},
			},
		},
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					nutanixprojectpolicy.CSIStorageProjectPolicyAnnotationKey: storagePolicy,
				},
			},
		},
	}
}

func TestNewProjectScopedAOSVersionChecks_SkippedWhenStoragePolicyDefault(t *testing.T) {
	t.Parallel()

	assert.Nil(t, newProjectScopedAOSVersionChecks(projectScopedAOSCheckDeps(
		nutanixprojectpolicy.StoragePolicyDefault,
	)))
}

func TestNewProjectScopedAOSVersionChecks_RunsWhenStoragePolicyProjectPolicy(t *testing.T) {
	t.Parallel()

	checks := newProjectScopedAOSVersionChecks(projectScopedAOSCheckDeps(
		nutanixprojectpolicy.StoragePolicyProjectPolicy,
	))
	require.NotEmpty(t, checks)
}

func TestNewProjectScopedAOSVersionChecks_GatedOnPC76(t *testing.T) {
	t.Parallel()

	cd := &checkDependencies{
		nclient:                  &clientWrapper{},
		pcVersion:                "7.5",
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{},
	}
	assert.Nil(t, newProjectScopedAOSVersionChecks(cd))
}

func TestProjectScopedAOSVersionCheck_NoOpWhenNotProjectScoped(t *testing.T) {
	t.Parallel()

	check := &aosVersionCheck{
		machineSpec:       machineDetailsWithCluster("pe-uuid"),
		field:             "field",
		name:              projectScopedAOSVersionCheckName,
		nclient:           &clientWrapper{}, // No API funcs: none should be called.
		minVersion:        minProjectScopedAOSVersion,
		pcVersion:         "7.5", // Below 7.6, so the cluster is never project-scoped.
		projectScopedOnly: true,
	}

	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.Empty(t, result.Causes)
	assert.Equal(t, projectScopedAOSVersionCheckName, check.Name())
}

func TestProjectScopedAOSVersionCheck_FailsBelow76(t *testing.T) {
	t.Parallel()

	check := &aosVersionCheck{
		machineSpec:       machineDetailsWithCluster("pe-uuid"),
		field:             "field",
		name:              projectScopedAOSVersionCheckName,
		nclient:           projectScopedClient(t, "pe-uuid", "7.5"),
		minVersion:        minProjectScopedAOSVersion,
		pcVersion:         "7.6",
		projectScopedOnly: true,
		projectRef: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierUUID,
			UUID: new("project-uuid"),
		},
	}

	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	require.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "project-scoped")
	assert.Contains(t, result.Causes[0].Message, minProjectScopedAOSVersion)
}

func TestProjectScopedAOSVersionCheck_PassesAt76(t *testing.T) {
	t.Parallel()

	check := &aosVersionCheck{
		machineSpec:       machineDetailsWithCluster("pe-uuid"),
		field:             "field",
		name:              projectScopedAOSVersionCheckName,
		nclient:           projectScopedClient(t, "pe-uuid", "7.6"),
		minVersion:        minProjectScopedAOSVersion,
		pcVersion:         "7.6",
		projectScopedOnly: true,
		projectRef: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierUUID,
			UUID: new("project-uuid"),
		},
	}

	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.Empty(t, result.Causes)
}
