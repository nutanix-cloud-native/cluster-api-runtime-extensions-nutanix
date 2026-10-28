// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
	preflightnutanix "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/nutanix"
)

const (
	projectPolicyKey = nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey
	projectUUIDKey   = "capx.nutanix.com/project-uuid"
	storagePolicyKey = nutanixprojectpolicy.CSIStorageProjectPolicyAnnotationKey
)

func customProjectRef() *capxv1.NutanixResourceIdentifier {
	return &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("custom-project"),
	}
}

// nutanixClusterConfig returns a minimal Nutanix clusterConfig, optionally declaring CSI and a control
// plane project reference.
func nutanixClusterConfig(withCSI bool, cpProject *capxv1.NutanixResourceIdentifier) *carenv1.NutanixClusterConfigSpec {
	cc := &carenv1.NutanixClusterConfigSpec{
		Nutanix: &carenv1.NutanixSpec{},
		ControlPlane: &carenv1.NutanixControlPlaneSpec{
			Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Project: cpProject,
				},
			},
		},
	}
	if withCSI {
		cc.Addons = &carenv1.NutanixAddons{CSI: &carenv1.NutanixCSI{}}
	}
	return cc
}

func clusterConfigVar(t *testing.T, cc *carenv1.NutanixClusterConfigSpec) clusterv1.ClusterVariable {
	t.Helper()
	v, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, cc)
	require.NoError(t, err)
	return *v
}

// clusterWithConfig builds a topology cluster carrying the given clusterConfig and optional annotations.
func clusterWithConfig(
	t *testing.T,
	cc *carenv1.NutanixClusterConfigSpec,
	annotations map[string]string,
) *clusterv1.Cluster {
	t.Helper()
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Namespace:   "default",
			Annotations: annotations,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef:  clusterv1.ClusterClassRef{Name: "test-class"},
				Version:   "v1.30.0",
				Variables: []clusterv1.ClusterVariable{clusterConfigVar(t, cc)},
			},
		},
	}
}

func newDefaulter(pcFn pcVersionResolver) *projectPolicyDefaulter {
	d := &projectPolicyDefaulter{
		pcVersionFn: pcFn,
	}
	// Keep test helper aligned with production constructor defaults so update-path
	// tests don't panic on nil resolver hooks.
	d.resolveProjectFn = func(
		_ context.Context,
		_ *clusterv1.Cluster,
		_ *carenv1.NutanixClusterConfigSpec,
		id *capxv1.NutanixResourceIdentifier,
		_ string,
	) (string, string, error) {
		if id != nil && id.UUID != nil && *id.UUID != "" {
			return *id.UUID, "", nil
		}
		if id != nil && id.Name != nil && *id.Name != "" {
			return *id.Name, *id.Name, nil
		}
		return "", "", nil
	}
	d.resolveDefaultProjectFn = func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
		return "default-project-uuid", "default", nil
	}
	return d
}

func withProjectResolvers(
	d *projectPolicyDefaulter,
	resolveProject func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, *capxv1.NutanixResourceIdentifier, string) (string, string, error),
	resolveDefault func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error),
) *projectPolicyDefaulter {
	d.resolveProjectFn = resolveProject
	d.resolveDefaultProjectFn = resolveDefault
	return d
}

func staticPC(version string) pcVersionResolver {
	return func(
		context.Context, ctrlclient.Client, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec,
	) (string, error) {
		return version, nil
	}
}

func erroringPC(msg string) pcVersionResolver {
	return func(
		context.Context, ctrlclient.Client, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec,
	) (string, error) {
		return "", fmt.Errorf("%s", msg)
	}
}

func req(op admissionv1.Operation) admission.Request {
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: op}}
}

func TestProjectPolicyDefaulter_PreservesExistingAnnotation(t *testing.T) {
	t.Parallel()

	d := newDefaulter(staticPC("7.6.0"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), map[string]string{
		projectPolicyKey: nutanixprojectpolicy.PolicyUnrestricted,
	})

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
	assert.Equal(t, nutanixprojectpolicy.PolicyUnrestricted, cluster.Annotations[projectPolicyKey],
		"existing annotation must be frozen")
}

func TestProjectPolicyDefaulter_CreatePassesPCVersionToResolver(t *testing.T) {
	t.Parallel()

	var got string
	d := newDefaulter(staticPC("7.5.0"))
	d.resolveProjectFn = func(
		_ context.Context,
		_ *clusterv1.Cluster,
		_ *carenv1.NutanixClusterConfigSpec,
		_ *capxv1.NutanixResourceIdentifier,
		pcVersion string,
	) (string, string, error) {
		got = pcVersion
		return "custom-uuid", "coke", nil
	}

	cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), nil)
	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
	assert.Equal(t, "7.5.0", got)
}

func TestProjectPolicyDefaulter_UpdatePassesPCVersionToResolver(t *testing.T) {
	t.Parallel()

	var got string
	d := newDefaulter(staticPC("7.5.0"))
	d.resolveProjectFn = func(
		_ context.Context,
		_ *clusterv1.Cluster,
		_ *carenv1.NutanixClusterConfigSpec,
		_ *capxv1.NutanixResourceIdentifier,
		pcVersion string,
	) (string, string, error) {
		got = pcVersion
		return "custom-uuid", "coke", nil
	}

	cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), nil)
	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
	assert.Equal(t, "7.5.0", got)
	assert.Equal(t, policySingleProject, cluster.Annotations[projectPolicyKey])
}

func TestProjectPolicyDefaulter_UpdateWithProjectRefsFailsClosedOnPCError(t *testing.T) {
	t.Parallel()

	d := newDefaulter(erroringPC("connection refused"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), nil)

	err := d.mutateCluster(context.Background(), req(admissionv1.Update), cluster)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve Prism Central version for project policy annotation")
	_, ok := cluster.Annotations[projectPolicyKey]
	assert.False(t, ok, "no annotation must be set when the policy cannot be determined")
}

func TestProjectPolicyDefaulter_SkipsDelete(t *testing.T) {
	t.Parallel()

	d := newDefaulter(staticPC("7.6.0"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), nil)

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Delete), cluster))
	_, ok := cluster.Annotations[projectPolicyKey]
	assert.False(t, ok)
}

// A paused cluster must be skipped without any Prism Central call, so pause/move/restore work while PC is
// unavailable. erroringPC fails if the PC resolver is consulted.
func TestProjectPolicyDefaulter_SkipsPausedWithoutPCCall(t *testing.T) {
	t.Parallel()

	for _, op := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update} {
		d := newDefaulter(erroringPC("pcVersionFn must not be called for paused clusters"))
		cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), nil)
		cluster.Spec.Paused = new(true)

		require.NoError(t, d.mutateCluster(context.Background(), req(op), cluster))
		_, ok := cluster.Annotations[projectPolicyKey]
		assert.False(t, ok, "paused cluster must not get an annotation set (operation=%s)", op)
	}
}

func TestProjectPolicyDefaulter_SkipsNonTopology(t *testing.T) {
	t.Parallel()

	d := newDefaulter(staticPC("7.6.0"))
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "default"},
	}

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
	_, ok := cluster.Annotations[projectPolicyKey]
	assert.False(t, ok)
}

// The create path selects the policy from the control-plane project alone, and does so identically on
// every Prism Central version: default-only for an absent or default project, single-project for any
// other project. Version differences live inside project resolution (v4 Projects on 7.6+, v3 below),
// which is why each case is asserted against both versions.
func TestProjectPolicyDefaulter_CreatePolicyFromControlPlaneProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		cpProject          *capxv1.NutanixResourceIdentifier
		resolveProjectUUID string
		resolveProjectName string
		resolveDefaultUUID string
		wantPolicy         string
		wantProjectUUID    string
	}{
		{
			name:       "no project => default-only",
			wantPolicy: nutanixprojectpolicy.PolicyDefaultOnly,
		},
		{
			name:               "custom project resolving to default => default-only",
			cpProject:          customProjectRef(),
			resolveProjectUUID: "default-uuid",
			resolveProjectName: "_internal",
			resolveDefaultUUID: "default-uuid",
			wantPolicy:         nutanixprojectpolicy.PolicyDefaultOnly,
		},
		{
			name:               "custom non-default project => single-project",
			cpProject:          customProjectRef(),
			resolveProjectUUID: "custom-uuid",
			resolveDefaultUUID: "default-uuid",
			wantPolicy:         "single-project",
			wantProjectUUID:    "custom-uuid",
		},
	}

	for _, tt := range tests {
		for _, pcVersion := range []string{"7.6.0", "7.5.0"} {
			t.Run(fmt.Sprintf("%s (PC %s)", tt.name, pcVersion), func(t *testing.T) {
				t.Parallel()
				d := withProjectResolvers(
					newDefaulter(staticPC(pcVersion)),
					func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, *capxv1.NutanixResourceIdentifier, string) (string, string, error) {
						return tt.resolveProjectUUID, tt.resolveProjectName, nil
					},
					func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
						if !nutanixprojectpolicy.PCVersionAtLeast76(pcVersion) {
							t.Fatal("resolveDefaultProjectFn must not be called on PC < 7.6")
						}
						return tt.resolveDefaultUUID, "", nil
					},
				)
				cluster := clusterWithConfig(t, nutanixClusterConfig(true, tt.cpProject), nil)

				require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
				assert.Equal(t, tt.wantPolicy, cluster.Annotations[projectPolicyKey])
				assert.Equal(t, tt.wantProjectUUID, cluster.Annotations[projectUUIDKey])
			})
		}
	}
}

// A Prism Central that cannot report a default project at all still yields single-project rather than
// silently treating the cluster's project as the default.
func TestProjectPolicyDefaulter_CreateWithoutDiscoverableDefaultIsSingleProject(t *testing.T) {
	t.Parallel()

	d := withProjectResolvers(
		newDefaulter(staticPC("7.5.0")),
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, *capxv1.NutanixResourceIdentifier, string) (string, string, error) {
			return "custom-uuid", "", nil
		},
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
			return "", "", errors.New("default project not discoverable")
		},
	)
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), nil)

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
	assert.Equal(t, policySingleProject, cluster.Annotations[projectPolicyKey])
	assert.Equal(t, "custom-uuid", cluster.Annotations[projectUUIDKey])
}

// unrestricted imposes no tenancy boundary, so it is the one value an update re-evaluates. Every other
// value is a committed boundary and stays frozen, and a create never re-evaluates at all.
func TestProjectPolicyDefaulter_ReevaluatesUnrestrictedOnUpdateOnly(t *testing.T) {
	t.Parallel()

	newCluster := func(t *testing.T, existing string) *clusterv1.Cluster {
		t.Helper()
		// No project anywhere, so re-evaluation resolves to default-only.
		c := clusterWithWorker(t, nutanixWorkerConfig(nil), nil)
		c.Annotations = map[string]string{projectPolicyKey: existing}
		return c
	}

	t.Run("unrestricted is recomputed on update", func(t *testing.T) {
		t.Parallel()
		d := newDefaulter(erroringPC("pcVersionFn must not be called on update"))
		cluster := newCluster(t, nutanixprojectpolicy.PolicyUnrestricted)

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
		assert.Equal(t, nutanixprojectpolicy.PolicyDefaultOnly, cluster.Annotations[projectPolicyKey])
	})

	t.Run("unrestricted is frozen on create", func(t *testing.T) {
		t.Parallel()
		d := newDefaulter(staticPC("7.6.0"))
		cluster := newCluster(t, nutanixprojectpolicy.PolicyUnrestricted)

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
		assert.Equal(t, nutanixprojectpolicy.PolicyUnrestricted, cluster.Annotations[projectPolicyKey])
	})

	for _, frozen := range []string{nutanixprojectpolicy.PolicyDefaultOnly, policySingleProject} {
		t.Run(frozen+" stays frozen on update", func(t *testing.T) {
			t.Parallel()
			d := newDefaulter(erroringPC("pcVersionFn must not be called on update"))
			cluster := newCluster(t, frozen)

			require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
			assert.Equal(t, frozen, cluster.Annotations[projectPolicyKey])
		})
	}
}

// CREATE ignores worker project refs: only the control-plane project selects the policy, on every
// Prism Central version.
func TestProjectPolicyDefaulter_CreateIgnoresWorkerProjects(t *testing.T) {
	t.Parallel()

	for _, pcVersion := range []string{"7.6.0", "7.5.0"} {
		t.Run("PC "+pcVersion, func(t *testing.T) {
			t.Parallel()
			d := newDefaulter(staticPC(pcVersion))
			cluster := clusterWithWorker(t, nutanixWorkerConfig(customProjectRef()), nil)

			require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
			assert.Equal(t, nutanixprojectpolicy.PolicyDefaultOnly, cluster.Annotations[projectPolicyKey],
				"worker-only project must not affect create; no CP project => default-only")
			assert.Empty(t, cluster.Annotations[projectUUIDKey])
		})
	}
}

// Upgrade (UPDATE) re-evaluates an existing unrestricted annotation using every Nutanix machine spec.
// This is the 2.18 -> 2.19 path for UI clusters that were stamped unrestricted.
func TestProjectPolicyDefaulter_UpgradeNarrowsUnrestricted(t *testing.T) {
	t.Parallel()

	defaultRef := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("default-uuid"),
	}
	otherRef := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("other-project"),
	}

	tests := []struct {
		name            string
		cpProject       *capxv1.NutanixResourceIdentifier
		workerProject   *capxv1.NutanixResourceIdentifier
		want            string
		wantProjectUUID string
	}{
		{
			name:          "no project anywhere => default-only",
			cpProject:     nil,
			workerProject: nil,
			want:          nutanixprojectpolicy.PolicyDefaultOnly,
		},
		{
			name:            "every spec names the same non-default project => single-project",
			cpProject:       customProjectRef(),
			workerProject:   customProjectRef(),
			want:            policySingleProject,
			wantProjectUUID: "custom-project",
		},
		{
			name:          "every spec names the default project => default-only",
			cpProject:     defaultRef,
			workerProject: defaultRef,
			want:          nutanixprojectpolicy.PolicyDefaultOnly,
		},
		{
			name:          "CP custom project, workers unset => unrestricted (hybrid)",
			cpProject:     customProjectRef(),
			workerProject: nil,
			want:          nutanixprojectpolicy.PolicyUnrestricted,
		},
		{
			name:          "CP unset, workers custom project => unrestricted (hybrid)",
			cpProject:     nil,
			workerProject: customProjectRef(),
			want:          nutanixprojectpolicy.PolicyUnrestricted,
		},
		{
			name:          "workers name the default/_internal project => default-only",
			cpProject:     nil,
			workerProject: defaultRef,
			want:          nutanixprojectpolicy.PolicyDefaultOnly,
		},
		{
			name:          "multiple distinct projects => unrestricted",
			cpProject:     customProjectRef(),
			workerProject: otherRef,
			want:          nutanixprojectpolicy.PolicyUnrestricted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := withProjectResolvers(
				newDefaulter(staticPC("7.6.0")),
				func(_ context.Context, _ *clusterv1.Cluster, _ *carenv1.NutanixClusterConfigSpec, id *capxv1.NutanixResourceIdentifier, _ string) (string, string, error) {
					if id == nil {
						return "", "", nil
					}
					if id.UUID != nil {
						return *id.UUID, "", nil
					}
					return "resolved-cp-uuid", "", nil
				},
				func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
					return "default-uuid", "", nil
				},
			)
			cluster := clusterWithWorker(t, nutanixWorkerConfig(tt.workerProject), nil)
			cluster.Spec.Topology.Variables[0] = clusterConfigVar(t, nutanixClusterConfig(true, tt.cpProject))
			cluster.Annotations = map[string]string{projectPolicyKey: nutanixprojectpolicy.PolicyUnrestricted}

			require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
			assert.Equal(t, tt.want, cluster.Annotations[projectPolicyKey])
			assert.Equal(t, tt.wantProjectUUID, cluster.Annotations[projectUUIDKey])
		})
	}
}

func TestProjectPolicyDefaulter_UpdateBackfill(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		cpProject       *capxv1.NutanixResourceIdentifier // for single-project backfill this wins by precedence
		workerProject   *capxv1.NutanixResourceIdentifier
		want            string
		wantProjectUUID string
	}{
		{
			name:          "no annotation + no projects => default-only",
			cpProject:     nil,
			workerProject: nil,
			want:          nutanixprojectpolicy.PolicyDefaultOnly,
		},
		{
			// Partial coverage: the worker pool names no project, so it runs in the default project
			// while the control plane runs in a custom one. The cluster spans two projects.
			name:          "no annotation + only the control plane names a non-default project => unrestricted",
			cpProject:     customProjectRef(),
			workerProject: nil,
			want:          nutanixprojectpolicy.PolicyUnrestricted,
		},
		{
			// Partial coverage, but the named project is the default one, so every machine ends up
			// in the default project regardless.
			name: "no annotation + only the control plane names the default project => default-only",
			cpProject: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("default-uuid"),
			},
			workerProject: nil,
			want:          nutanixprojectpolicy.PolicyDefaultOnly,
		},
		{
			name:            "no annotation + every spec names the same non-default project => single-project",
			cpProject:       customProjectRef(),
			workerProject:   customProjectRef(),
			want:            "single-project",
			wantProjectUUID: "custom-project",
		},
		{
			// Full coverage, but the named project is the default one. This must agree with the
			// partial-coverage case above rather than pinning single-project to the default project.
			name: "no annotation + every spec names the default project => default-only",
			cpProject: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("default-uuid"),
			},
			workerProject: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("default-uuid"),
			},
			want: nutanixprojectpolicy.PolicyDefaultOnly,
		},
		{
			name:      "no annotation + multiple projects => unrestricted",
			cpProject: customProjectRef(),
			workerProject: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("other-project"),
			},
			want: nutanixprojectpolicy.PolicyUnrestricted,
		},
		{
			name: "no annotation + project and default project => unrestricted",
			cpProject: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("custom-project"),
			},
			workerProject: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: new("default-project-uuid"),
			},
			want: nutanixprojectpolicy.PolicyUnrestricted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := withProjectResolvers(
				newDefaulter(staticPC("7.6.0")),
				func(_ context.Context, _ *clusterv1.Cluster, _ *carenv1.NutanixClusterConfigSpec, id *capxv1.NutanixResourceIdentifier, _ string) (string, string, error) {
					if id == nil {
						return "", "", nil
					}
					if id.UUID != nil {
						return *id.UUID, "", nil
					}
					return "resolved-cp-uuid", "", nil
				},
				func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
					return "default-uuid", "", nil
				},
			)
			cluster := clusterWithWorker(t, nutanixWorkerConfig(tt.workerProject), nil)
			cluster.Spec.Topology.Variables[0] = clusterConfigVar(t, nutanixClusterConfig(true, tt.cpProject))

			require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
			assert.Equal(t, tt.want, cluster.Annotations[projectPolicyKey])
			assert.Equal(t, tt.wantProjectUUID, cluster.Annotations[projectUUIDKey])
		})
	}
}

func TestProjectPolicyDefaulter_Brownfield_PreservesExistingAnnotation(t *testing.T) {
	t.Parallel()
	d := withProjectResolvers(
		newDefaulter(erroringPC("pcVersionFn should not be called on update")),
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, *capxv1.NutanixResourceIdentifier, string) (string, string, error) {
			return "resolved", "", nil
		},
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
			return "default", "", nil
		},
	)
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), map[string]string{
		projectPolicyKey: "single-project",
		projectUUIDKey:   "frozen-uuid",
	})

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
	assert.Equal(t, "single-project", cluster.Annotations[projectPolicyKey])
	assert.Equal(t, "frozen-uuid", cluster.Annotations[projectUUIDKey])
}

// If the default project cannot be read, UPDATE does not fail closed: the unique project is treated
// as not-default, then coverage selects single-project or unrestricted.
func TestProjectPolicyDefaulter_UpdateDefaultProjectNotAccessible(t *testing.T) {
	t.Parallel()

	inaccessibleDefault := func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
		return "", "", fmt.Errorf("%w: default project", preflightnutanix.ErrProjectNotAccessible)
	}
	resolveByUUID := func(_ context.Context, _ *clusterv1.Cluster, _ *carenv1.NutanixClusterConfigSpec, id *capxv1.NutanixResourceIdentifier, _ string) (string, string, error) {
		if id != nil && id.UUID != nil {
			return *id.UUID, "", nil
		}
		return "", "", nil
	}

	t.Run("every spec names the same project => single-project", func(t *testing.T) {
		t.Parallel()
		d := withProjectResolvers(
			newDefaulter(staticPC("7.6.0")),
			resolveByUUID,
			inaccessibleDefault,
		)
		cluster := clusterWithWorker(t, nutanixWorkerConfig(customProjectRef()), nil)
		cluster.Spec.Topology.Variables[0] = clusterConfigVar(t, nutanixClusterConfig(true, customProjectRef()))

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
		assert.Equal(t, policySingleProject, cluster.Annotations[projectPolicyKey])
		assert.Equal(t, "custom-project", cluster.Annotations[projectUUIDKey])
	})

	t.Run("partial coverage => unrestricted", func(t *testing.T) {
		t.Parallel()
		d := withProjectResolvers(
			newDefaulter(staticPC("7.6.0")),
			resolveByUUID,
			inaccessibleDefault,
		)
		cluster := clusterWithWorker(t, nutanixWorkerConfig(nil), nil)
		cluster.Spec.Topology.Variables[0] = clusterConfigVar(t, nutanixClusterConfig(true, customProjectRef()))

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
		assert.Equal(t, nutanixprojectpolicy.PolicyUnrestricted, cluster.Annotations[projectPolicyKey])
		assert.Empty(t, cluster.Annotations[projectUUIDKey])
	})
}

func TestProjectPolicyDefaulter_CreatePC76_NoDefaultVisibleFallsBackToSingleProject(t *testing.T) {
	t.Parallel()
	d := withProjectResolvers(
		newDefaulter(staticPC("7.6.0")),
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, *capxv1.NutanixResourceIdentifier, string) (string, string, error) {
			return "custom-uuid", "", nil
		},
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
			return "", "", errors.New("default project lookup failed")
		},
	)
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), nil)

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
	assert.Equal(t, "single-project", cluster.Annotations[projectPolicyKey])
	assert.Equal(t, "custom-uuid", cluster.Annotations[projectUUIDKey])
}

// CSI presence must not change the decision: a cluster without the CSI addon on PC >= 7.6 still gets
// default-only, identical to a cluster with CSI declared.
func TestProjectPolicyDefaulter_CSIPresenceIrrelevant(t *testing.T) {
	t.Parallel()

	for _, withCSI := range []bool{true, false} {
		d := newDefaulter(staticPC("7.6.0"))
		cluster := clusterWithConfig(t, nutanixClusterConfig(withCSI, nil), nil)

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
		assert.Equal(t, nutanixprojectpolicy.PolicyDefaultOnly, cluster.Annotations[projectPolicyKey],
			"CSI presence (withCSI=%v) must not affect the policy", withCSI)
	}
}

func TestProjectPolicyDefaulter_FailsClosedOnPCError(t *testing.T) {
	t.Parallel()

	d := newDefaulter(erroringPC("prism central unreachable"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), nil)

	err := d.mutateCluster(context.Background(), req(admissionv1.Create), cluster)
	require.Error(t, err)
	_, ok := cluster.Annotations[projectPolicyKey]
	assert.False(t, ok, "annotation must not be set when the decision cannot be made")
}

// A panic in the Prism Central resolution path must be recovered and returned as a normal fail-closed
// error, never crash the webhook. The generated Nutanix SDK's "interface conversion: *http.Response, not
// []uint8" panic (raised on a non-JSON PC response, typically a rejected-credentials 401) must surface as
// an actionable credentials-oriented message, while any other panic keeps the generic report-it message.
func TestProjectPolicyDefaulter_RecoversFromResolverPanic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		panicValue  any
		wantContain string
		wantMissing string
	}{
		{
			name:        "non-JSON PC response looks like an auth failure",
			panicValue:  "interface conversion: interface {} is *http.Response, not []uint8",
			wantContain: "unexpected non-JSON response",
		},
		{
			name:        "other panics keep the generic internal-error message",
			panicValue:  "some unrelated nil pointer dereference",
			wantContain: "internal error (panic)",
			wantMissing: "unexpected non-JSON response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := newDefaulter(staticPC("7.6.0"))
			d.resolveProjectFn = func(
				context.Context,
				*clusterv1.Cluster,
				*carenv1.NutanixClusterConfigSpec,
				*capxv1.NutanixResourceIdentifier,
				string,
			) (string, string, error) {
				panic(tt.panicValue)
			}
			cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), nil)

			var err error
			require.NotPanics(t, func() {
				err = d.mutateCluster(context.Background(), req(admissionv1.Create), cluster)
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantContain)
			if tt.wantMissing != "" {
				assert.NotContains(t, err.Error(), tt.wantMissing)
			}
			_, ok := cluster.Annotations[projectPolicyKey]
			assert.False(t, ok, "annotation must not be set when resolution panics")
		})
	}
}

func nutanixWorkerConfig(project *capxv1.NutanixResourceIdentifier) *carenv1.NutanixWorkerNodeConfigSpec {
	return &carenv1.NutanixWorkerNodeConfigSpec{
		Nutanix: &carenv1.NutanixWorkerNodeSpec{
			MachineDetails: carenv1.NutanixMachineDetails{Project: project},
		},
	}
}

func workerConfigVar(t *testing.T, ws *carenv1.NutanixWorkerNodeConfigSpec) clusterv1.ClusterVariable {
	t.Helper()
	v, err := variables.MarshalToClusterVariable(carenv1.WorkerConfigVariableName, ws)
	require.NoError(t, err)
	return *v
}

// clusterWithWorker builds a topology cluster (control plane has no project) with a single machine
// deployment, an optional top-level workerConfig, and an optional machine-deployment override workerConfig.
func clusterWithWorker(
	t *testing.T,
	topLevelWorker *carenv1.NutanixWorkerNodeConfigSpec,
	mdOverride *clusterv1.ClusterVariable,
) *clusterv1.Cluster {
	t.Helper()
	vars := []clusterv1.ClusterVariable{clusterConfigVar(t, nutanixClusterConfig(true, nil))}
	if topLevelWorker != nil {
		vars = append(vars, workerConfigVar(t, topLevelWorker))
	}
	md := clusterv1.MachineDeploymentTopology{
		Class:     "default-worker",
		Name:      "md-0",
		Variables: clusterv1.MachineDeploymentVariables{Overrides: []clusterv1.ClusterVariable{}},
	}
	if mdOverride != nil {
		md.Variables.Overrides = append(md.Variables.Overrides, *mdOverride)
	}
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "default"},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef:  clusterv1.ClusterClassRef{Name: "test-class"},
				Version:   "v1.30.0",
				Variables: vars,
				Workers: clusterv1.WorkersTopology{
					MachineDeployments: []clusterv1.MachineDeploymentTopology{md},
				},
			},
		},
	}
}

// Worker-only project references are part of UPDATE backfill decision:
// one resolved project => single-project, multiple => unrestricted.
func TestProjectPolicyDefaulter_WorkerProjectInUse(t *testing.T) {
	t.Parallel()

	// clusterWithWorker leaves the control plane without a project, so a worker project makes these
	// partial-coverage clusters: the control plane runs in the default project, the worker elsewhere.
	t.Run("top-level workerConfig project, control plane unset => unrestricted", func(t *testing.T) {
		t.Parallel()
		d := newDefaulter(staticPC("7.6.0"))
		cluster := clusterWithWorker(t, nutanixWorkerConfig(customProjectRef()), nil)

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
		assert.Equal(t, nutanixprojectpolicy.PolicyUnrestricted, cluster.Annotations[projectPolicyKey])
		assert.Empty(t, cluster.Annotations[projectUUIDKey])
	})

	t.Run("machine-deployment override workerConfig project, control plane unset => unrestricted", func(t *testing.T) {
		t.Parallel()
		override := workerConfigVar(t, nutanixWorkerConfig(customProjectRef()))
		d := newDefaulter(staticPC("7.6.0"))
		cluster := clusterWithWorker(t, nil, &override)

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
		assert.Equal(t, nutanixprojectpolicy.PolicyUnrestricted, cluster.Annotations[projectPolicyKey])
		assert.Empty(t, cluster.Annotations[projectUUIDKey])
	})

	t.Run("worker names the default project, control plane unset => default-only", func(t *testing.T) {
		t.Parallel()
		d := newDefaulter(staticPC("7.6.0"))
		cluster := clusterWithWorker(t, nutanixWorkerConfig(&capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierUUID,
			UUID: new("default-project-uuid"),
		}), nil)

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
		assert.Equal(t, nutanixprojectpolicy.PolicyDefaultOnly, cluster.Annotations[projectPolicyKey])
	})

	t.Run("no project anywhere => default-only", func(t *testing.T) {
		t.Parallel()
		d := newDefaulter(erroringPC("pcVersionFn must not be called on update"))
		cluster := clusterWithWorker(t, nutanixWorkerConfig(nil), nil)

		require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
		assert.Equal(t, nutanixprojectpolicy.PolicyDefaultOnly, cluster.Annotations[projectPolicyKey])
	})
}

func TestProjectPolicyDefaulter_CreatePC76_FailsClosedOnResolveProjectError(t *testing.T) {
	t.Parallel()
	d := withProjectResolvers(
		newDefaulter(staticPC("7.6.0")),
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, *capxv1.NutanixResourceIdentifier, string) (string, string, error) {
			return "", "", errors.New("project resolve failed")
		},
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
			return "default-project-uuid", "default", nil
		},
	)
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, customProjectRef()), nil)

	err := d.mutateCluster(context.Background(), req(admissionv1.Create), cluster)
	require.Error(t, err)
	_, ok := cluster.Annotations[projectPolicyKey]
	assert.False(t, ok)
}

func TestProjectPolicyDefaulter_UpdateBackfill_FailsClosedOnEmptyResolvedUUID(t *testing.T) {
	t.Parallel()
	d := withProjectResolvers(
		newDefaulter(staticPC("7.6.0")),
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, *capxv1.NutanixResourceIdentifier, string) (string, string, error) {
			return "", "", nil
		},
		func(context.Context, *clusterv1.Cluster, *carenv1.NutanixClusterConfigSpec, string) (string, string, error) {
			return "default-project-uuid", "default", nil
		},
	)
	cluster := clusterWithWorker(t, nutanixWorkerConfig(customProjectRef()), nil)

	err := d.mutateCluster(context.Background(), req(admissionv1.Update), cluster)
	require.Error(t, err)
	_, ok := cluster.Annotations[projectPolicyKey]
	assert.False(t, ok)
}

// A malformed workerConfig must cause the defaulter to fail closed rather than silently treat the cluster
// as having no project.
func TestProjectPolicyDefaulter_FailsClosedOnBadWorkerConfig(t *testing.T) {
	t.Parallel()

	badWorkerConfig := clusterv1.ClusterVariable{
		Name:  carenv1.WorkerConfigVariableName,
		Value: apiextensionsv1.JSON{Raw: []byte("{")},
	}
	d := newDefaulter(erroringPC("pcVersionFn must not be called on update"))
	cluster := clusterWithWorker(t, nil, &badWorkerConfig)

	err := d.mutateCluster(context.Background(), req(admissionv1.Update), cluster)
	require.Error(t, err)
	_, ok := cluster.Annotations[projectPolicyKey]
	assert.False(t, ok, "annotation must not be set when a worker config cannot be parsed")
}

func TestSetStoragePolicy_CreateLeavesExisting(t *testing.T) {
	t.Parallel()

	d := newDefaulter(staticPC("7.6.0"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), map[string]string{
		storagePolicyKey: nutanixprojectpolicy.StoragePolicyDefault,
	})

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
	assert.Equal(t, nutanixprojectpolicy.StoragePolicyDefault, cluster.Annotations[storagePolicyKey])
}

func TestSetStoragePolicy_CreateSetsProjectPolicyWhenAbsent(t *testing.T) {
	t.Parallel()

	d := newDefaulter(staticPC("7.6.0"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), nil)

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Create), cluster))
	assert.Equal(t, nutanixprojectpolicy.StoragePolicyProjectPolicy, cluster.Annotations[storagePolicyKey])
}

func TestSetStoragePolicy_PausedSkips(t *testing.T) {
	t.Parallel()

	d := newDefaulter(erroringPC("pcVersionFn must not be called for paused clusters"))

	for _, op := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update} {
		t.Run(string(op), func(t *testing.T) {
			t.Parallel()
			cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), nil)
			cluster.Spec.Paused = new(true)

			require.NoError(t, d.mutateCluster(context.Background(), req(op), cluster))
			_, ok := cluster.Annotations[storagePolicyKey]
			assert.False(t, ok)
		})
	}
}

func TestSetStoragePolicy_UpdateAbsentSetsDefault(t *testing.T) {
	t.Parallel()

	d := newDefaulter(erroringPC("pcVersionFn must not be called on update"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), nil)

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
	assert.Equal(t, nutanixprojectpolicy.StoragePolicyDefault, cluster.Annotations[storagePolicyKey])
}

func TestSetStoragePolicy_UpdateLeavesExisting(t *testing.T) {
	t.Parallel()

	d := newDefaulter(erroringPC("pcVersionFn must not be called on update"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), map[string]string{
		storagePolicyKey: nutanixprojectpolicy.StoragePolicyProjectPolicy,
	})

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
	assert.Equal(t, nutanixprojectpolicy.StoragePolicyProjectPolicy, cluster.Annotations[storagePolicyKey])
}

func TestSetStoragePolicy_FrozenCAPXPolicyStillSetsStoragePolicy(t *testing.T) {
	t.Parallel()

	d := newDefaulter(erroringPC("pcVersionFn must not be called when CAPX policy is frozen"))
	cluster := clusterWithConfig(t, nutanixClusterConfig(true, nil), map[string]string{
		projectPolicyKey: nutanixprojectpolicy.PolicyDefaultOnly,
	})

	require.NoError(t, d.mutateCluster(context.Background(), req(admissionv1.Update), cluster))
	assert.Equal(t, nutanixprojectpolicy.PolicyDefaultOnly, cluster.Annotations[projectPolicyKey])
	assert.Equal(t, nutanixprojectpolicy.StoragePolicyDefault, cluster.Annotations[storagePolicyKey])
}
