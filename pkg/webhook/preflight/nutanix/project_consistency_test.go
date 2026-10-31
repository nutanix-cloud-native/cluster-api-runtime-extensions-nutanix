// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
)

func TestProjectConsistencyPreflight_DeniesUnrestrictedDifferentProjectReferences(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Spec.Topology.ClassRef.Name = "nkp-nutanix-v2.19.0"
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyUnrestricted,
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsUUID() {
				return &resolvedProject{UUID: "project-a", Name: "project-a"}, nil
			}
			return &resolvedProject{UUID: "project-b", Name: "project-b"}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.Len(t, result.Causes, 2)
	for _, cause := range result.Causes {
		assert.Contains(t, cause.Message, "Cluster is spread across multiple Prism Central projects")
		assert.Contains(t, cause.Message, "same Prism Central project")
		assert.Contains(t, cause.Field, "machineDetails.project")
	}
}

func TestProjectConsistencyPreflight_AllowsUnrestrictedDifferentProjectReferencesOn218(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Spec.Topology.ClassRef.Name = "nkp-nutanix-v2.18.8"
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyUnrestricted,
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsUUID() {
				return &resolvedProject{UUID: "project-a", Name: "project-a"}, nil
			}
			return &resolvedProject{UUID: "project-b", Name: "project-b"}, nil
		},
	}

	result := (&projectConsistencyCheck{cd: cd}).Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Empty(t, result.Causes)
}

func TestProjectConsistencyPreflight_AllowsUnrestrictedMatchingProjectReferences(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Spec.Topology.ClassRef.Name = "nkp-nutanix-v2.19.0"
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyUnrestricted,
	}
	cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName["md-1"].Nutanix.MachineDetails.Project = &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("project-by-name"),
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "shared-project", Name: "shared"}, nil
		},
	}

	result := (&projectConsistencyCheck{cd: cd}).Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Empty(t, result.Causes)
}

func TestProjectConsistencyPreflight_DeniesUnrestrictedExplicitProjectAndDefaultProject(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Spec.Topology.ClassRef.Name = "nkp-nutanix-v2.19.0"
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyUnrestricted,
	}
	cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName["md-1"].Nutanix.MachineDetails.Project = nil
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "non-default-project", Name: "non-default"}, nil
		},
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			t.Fatal("ResolvePCDefaultProject must not be called for unrestricted policy")
			return nil, nil
		},
	}

	result := (&projectConsistencyCheck{cd: cd}).Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.Len(t, result.Causes, 2)
	for _, cause := range result.Causes {
		assert.Contains(t, cause.Message, "Cluster is spread across multiple Prism Central projects")
	}
}

func TestProjectConsistencyPreflight_AllowsUnrestrictedDefaultProjectReferences(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Spec.Topology.ClassRef.Name = "nkp-nutanix-v2.19.0"
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyUnrestricted,
	}
	cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Project = nil
	cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName["md-1"].Nutanix.MachineDetails.Project = nil

	result := (&projectConsistencyCheck{cd: cd}).Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Empty(t, result.Causes)
}

func TestProjectConsistencyPreflight_DeniesWhenProjectNotFound(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyUnrestricted,
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsName() {
				return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, id.DisplayString())
			}
			return &resolvedProject{UUID: *id.UUID, Name: "ok"}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "not found")
}

func TestProjectConsistencyPreflight_DeniesDefaultOnlyPolicyMismatch(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.pcVersion = "7.6"
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyDefaultOnly,
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsUUID() {
				return &resolvedProject{UUID: *id.UUID, Name: "other"}, nil
			}
			return &resolvedProject{UUID: "other-worker", Name: *id.Name}, nil
		},
		ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
			return &PCDefaultProject{UUID: "default-uuid", Name: "default"}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
}

func TestProjectConsistencyPreflight_DeniesSingleProjectPolicyMismatch(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: policySingleProject,
		singleProjectUUIDAnnotationKey:                         "00000000-0000-0000-0000-000000000999",
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsUUID() {
				return &resolvedProject{UUID: *id.UUID, Name: "other"}, nil
			}
			return &resolvedProject{UUID: "other-worker", Name: *id.Name}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "with uuid 00000000-0000-0000-0000-000000000999")
}

func TestProjectConsistencyPreflight_AllowsSingleProjectPolicyMatch(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: policySingleProject,
		singleProjectUUIDAnnotationKey:                         "00000000-0000-0000-0000-000000000777",
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "00000000-0000-0000-0000-000000000777", Name: "target"}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.InternalError)
	assert.Empty(t, result.Causes)
}

func TestProjectConsistencyPreflight_DeniesWhenProjectNotAccessible(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyUnrestricted,
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsName() {
				return nil, fmt.Errorf("%w: %s", ErrProjectNotAccessible, id.DisplayString())
			}
			return &resolvedProject{UUID: *id.UUID, Name: "ok"}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "not accessible")
}

func TestProjectConsistencyPreflight_DeniesSingleProjectPolicyMissingUUID(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: policySingleProject,
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsUUID() {
				return &resolvedProject{UUID: *id.UUID, Name: "ok"}, nil
			}
			return &resolvedProject{UUID: "worker-uuid", Name: *id.Name}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
	assert.Equal(t, singleProjectUUIDAnnotationField, result.Causes[0].Field)
}

func TestProjectConsistencyPreflight_DeniesSingleProjectPolicyInvalidUUID(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: policySingleProject,
		singleProjectUUIDAnnotationKey:                         "not-a-uuid",
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsUUID() {
				return &resolvedProject{UUID: *id.UUID, Name: "ok"}, nil
			}
			return &resolvedProject{UUID: "worker-uuid", Name: *id.Name}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
	assert.Equal(t, singleProjectUUIDAnnotationField, result.Causes[0].Field)
	assert.Contains(t, result.Causes[0].Message, "missing/invalid")
}

func TestGetActiveProjectPolicy(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	policy, target, err := getActiveProjectPolicy(cd)
	require.Error(t, err)
	assert.Empty(t, policy)
	assert.Empty(t, target)

	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyDefaultOnly,
	}
	policy, target, err = getActiveProjectPolicy(cd)
	require.NoError(t, err)
	assert.Equal(t, nutanixprojectpolicy.PolicyDefaultOnly, policy)
	assert.Empty(t, target)

	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: policySingleProject,
		singleProjectUUIDAnnotationKey:                         "00000000-0000-0000-0000-000000000111",
	}
	policy, target, err = getActiveProjectPolicy(cd)
	require.NoError(t, err)
	assert.Equal(t, policySingleProject, policy)
	assert.Equal(t, "00000000-0000-0000-0000-000000000111", target)
}

func TestProjectConsistencyPreflight_DeniesMissingPolicyAnnotation(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsUUID() {
				return &resolvedProject{UUID: *id.UUID, Name: "ok"}, nil
			}
			return &resolvedProject{UUID: "worker-uuid", Name: *id.Name}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
	assert.Equal(t, projectPolicyAnnotationField, result.Causes[0].Field)
}

func TestProjectConsistencyPreflight_AllowsNoRefsForDefaultOnlyPolicy(t *testing.T) {
	t.Parallel()

	for _, pcVersion := range []string{"7.5", "7.6", "master"} {
		t.Run("PC "+pcVersion, func(t *testing.T) {
			t.Parallel()
			cd := testCheckDependenciesWithProjects()
			cd.pcVersion = pcVersion
			cd.cluster.Annotations = map[string]string{
				nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyDefaultOnly,
			}
			cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Project = nil
			cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName["md-1"].Nutanix.MachineDetails.Project = nil
			cd.nclient = &clientWrapper{
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					t.Fatal("ResolvePCDefaultProject must not be called when default-only has no project refs")
					return nil, nil
				},
			}

			check := &projectConsistencyCheck{cd: cd}
			result := check.Run(context.Background())
			assert.True(t, result.Allowed)
			assert.False(t, result.InternalError)
			assert.Empty(t, result.Causes)
		})
	}
}

func TestProjectConsistencyPreflight_AllowsDefaultOnlyWhenRefsMatchDefault(t *testing.T) {
	t.Parallel()

	for _, pcVersion := range []string{"7.5", "7.6", "master"} {
		t.Run("PC "+pcVersion, func(t *testing.T) {
			t.Parallel()
			cd := testCheckDependenciesWithProjects()
			cd.pcVersion = pcVersion
			cd.cluster.Annotations = map[string]string{
				nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyDefaultOnly,
			}
			cd.nclient = &clientWrapper{
				ResolveProjectFunc: func(_ context.Context, _ *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
					return &resolvedProject{UUID: "internal-uuid", Name: v3DefaultProjectName}, nil
				},
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					if !nutanixprojectpolicy.PCVersionAtLeast76(pcVersion) {
						t.Fatal("ResolvePCDefaultProject must not be called on PC < 7.6")
					}
					return &PCDefaultProject{UUID: "internal-uuid", Name: v3DefaultProjectName}, nil
				},
			}

			check := &projectConsistencyCheck{cd: cd}
			result := check.Run(context.Background())
			assert.True(t, result.Allowed)
			assert.False(t, result.InternalError)
			assert.Empty(t, result.Causes)
		})
	}
}

func TestProjectConsistencyPreflight_DeniesDefaultOnlyWhenRefsDoNotMatchDefault(t *testing.T) {
	t.Parallel()

	for _, pcVersion := range []string{"7.5"} {
		t.Run("PC "+pcVersion, func(t *testing.T) {
			t.Parallel()
			cd := testCheckDependenciesWithProjects()
			cd.pcVersion = pcVersion
			cd.cluster.Annotations = map[string]string{
				nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: nutanixprojectpolicy.PolicyDefaultOnly,
			}
			cd.nclient = &clientWrapper{
				ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
					if id.IsUUID() {
						return &resolvedProject{UUID: *id.UUID, Name: "other"}, nil
					}
					return &resolvedProject{UUID: "other-worker", Name: *id.Name}, nil
				},
				ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
					t.Fatal("ResolvePCDefaultProject must not be called without v4 GetDefaultProject")
					return nil, nil
				},
			}

			check := &projectConsistencyCheck{cd: cd}
			result := check.Run(context.Background())
			assert.False(t, result.Allowed)
			assert.False(t, result.InternalError)
			require.NotEmpty(t, result.Causes)
		})
	}
}

func TestProjectConsistencyPreflight_DeniesNoRefsForSingleProjectPolicy(t *testing.T) {
	t.Parallel()

	const pinnedUUID = "00000000-0000-0000-0000-000000000111"
	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: policySingleProject,
		singleProjectUUIDAnnotationKey:                         pinnedUUID,
	}
	cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Project = nil
	cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName["md-1"].Nutanix.MachineDetails.Project = nil
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, _ *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "unused", Name: "unused"}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.Len(t, result.Causes, 2)
	for _, cause := range result.Causes {
		assert.Contains(t, cause.Message, "with uuid "+pinnedUUID)
		assert.Contains(t, cause.Message, "A missing project reference uses the Prism Central default project")
	}
}

func TestProjectConsistencyPreflight_DeniesSingleProjectWhenWorkerMissingRef(t *testing.T) {
	t.Parallel()

	const pinnedUUID = "00000000-0000-0000-0000-000000000777"
	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: policySingleProject,
		singleProjectUUIDAnnotationKey:                         pinnedUUID,
	}
	cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName["md-1"].Nutanix.MachineDetails.Project = nil
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, _ *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			return &resolvedProject{UUID: pinnedUUID, Name: "target"}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "with uuid "+pinnedUUID)
	assert.Contains(t, result.Causes[0].Message, "A missing project reference uses the Prism Central default project")
	assert.Contains(t, result.Causes[0].Field, "machineDetails.project")
}

func TestProjectConsistencyPreflight_DeniesUnsupportedPolicyValue(t *testing.T) {
	t.Parallel()

	cd := testCheckDependenciesWithProjects()
	cd.cluster.Annotations = map[string]string{
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey: "unsupported-policy",
	}
	cd.nclient = &clientWrapper{
		ResolveProjectFunc: func(_ context.Context, id *capxv1.NutanixResourceIdentifier, _ string) (*resolvedProject, error) {
			if id.IsUUID() {
				return &resolvedProject{UUID: *id.UUID, Name: "ok"}, nil
			}
			return &resolvedProject{UUID: "worker-uuid", Name: *id.Name}, nil
		},
	}

	check := &projectConsistencyCheck{cd: cd}
	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	assert.False(t, result.InternalError)
	require.NotEmpty(t, result.Causes)
	assert.Equal(t, projectPolicyAnnotationField, result.Causes[0].Field)
	assert.Contains(t, result.Causes[0].Message, "not supported")
}

func testCheckDependenciesWithProjects() *checkDependencies {
	return &checkDependencies{
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
			Spec: clusterv1.ClusterSpec{
				Topology: clusterv1.Topology{
					Workers: clusterv1.WorkersTopology{
						MachineDeployments: []clusterv1.MachineDeploymentTopology{
							{Name: "md-1"},
						},
					},
				},
			},
		},
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					MachineDetails: carenv1.NutanixMachineDetails{
						Project: &capxv1.NutanixResourceIdentifier{
							Type: capxv1.NutanixIdentifierUUID,
							UUID: new("control-plane-uuid"),
						},
					},
				},
			},
		},
		nutanixWorkerNodeConfigSpecByMachineDeploymentName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
			"md-1": {
				Nutanix: &carenv1.NutanixWorkerNodeSpec{
					MachineDetails: carenv1.NutanixMachineDetails{
						Project: &capxv1.NutanixResourceIdentifier{
							Type: capxv1.NutanixIdentifierName,
							Name: new("worker-project"),
						},
					},
				},
			},
		},
		log: logr.Discard(),
	}
}
