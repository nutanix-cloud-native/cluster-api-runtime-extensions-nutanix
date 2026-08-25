// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func TestIsMetroFailureDomain(t *testing.T) {
	t.Parallel()

	assert.True(t, IsMetroFailureDomain(MetroFailureDomainPrefix+"metro-1"))
	assert.True(t, IsMetroFailureDomain(MetroSiteFailureDomainPrefix+"site-1"))
	assert.False(t, IsMetroFailureDomain("fd-1"))
	assert.False(t, IsMetroFailureDomain(""))
}

func TestIsMetroCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cluster  *clusterv1.Cluster
		expected bool
	}{
		{
			name:     "nil cluster",
			cluster:  nil,
			expected: false,
		},
		{
			name: "no topology",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "no-topology"},
			},
			expected: false,
		},
		{
			name: "control plane metro failure domain",
			cluster: clusterWithConfigAndWorkers(
				&variables.ClusterConfigSpec{
					ControlPlane: &variables.ControlPlaneSpec{
						Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
							FailureDomains: []string{MetroFailureDomainPrefix + "metro-1"},
						},
					},
				},
				"",
			),
			expected: true,
		},
		{
			name: "worker metro failure domain",
			cluster: clusterWithConfigAndWorkers(
				&variables.ClusterConfigSpec{},
				MetroSiteFailureDomainPrefix+"site-1",
			),
			expected: true,
		},
		{
			name: "non-metro failure domains",
			cluster: clusterWithConfigAndWorkers(
				&variables.ClusterConfigSpec{
					ControlPlane: &variables.ControlPlaneSpec{
						Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
							FailureDomains: []string{"pe-fd-1"},
						},
					},
				},
				"pe-fd-2",
			),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, IsMetroCluster(tt.cluster))
		})
	}
}

func clusterWithConfigAndWorkers(
	cfg *variables.ClusterConfigSpec,
	workerFD string,
) *clusterv1.Cluster {
	raw, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "dummy-class"},
				Version:  "v1.30.0",
				Variables: []clusterv1.ClusterVariable{{
					Name:  v1alpha1.ClusterConfigVariableName,
					Value: apiextensionsv1.JSON{Raw: raw},
				}},
			},
		},
	}
	if workerFD != "" {
		cluster.Spec.Topology.Workers.MachineDeployments = []clusterv1.MachineDeploymentTopology{{
			Name:          "md-0",
			Class:         "default-worker",
			FailureDomain: workerFD,
		}}
	}
	return cluster
}

func TestDefaultMetroCSIComputeAffinity(t *testing.T) {
	t.Parallel()

	t.Run("adds DISABLED when metro and parameter omitted", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(t, []string{MetroFailureDomainPrefix + "metro-1"}, "", nil)

		mutated, err := DefaultMetroCSIComputeAffinity(cluster)
		require.NoError(t, err)
		assert.True(t, mutated)
		assert.Equal(t, ComputeAffinityDisabled, storageClassComputeAffinity(t, cluster, "volume"))
	})

	t.Run("adds DISABLED to every storage class missing the parameter", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(
			t,
			[]string{MetroFailureDomainPrefix + "metro-1"},
			"",
			map[string]map[string]string{
				"volume": {"storageContainer": "default-ctr"},
				"fast":   {"storageContainer": "ctr"},
			},
		)

		mutated, err := DefaultMetroCSIComputeAffinity(cluster)
		require.NoError(t, err)
		assert.True(t, mutated)
		assert.Equal(t, ComputeAffinityDisabled, storageClassComputeAffinity(t, cluster, "volume"))
		assert.Equal(t, ComputeAffinityDisabled, storageClassComputeAffinity(t, cluster, "fast"))
	})

	t.Run("does not overwrite explicit DISABLED", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(
			t,
			[]string{MetroFailureDomainPrefix + "metro-1"},
			"",
			map[string]map[string]string{
				"volume": {ComputeAffinityParameter: ComputeAffinityDisabled, "storageContainer": "ctr"},
			},
		)

		mutated, err := DefaultMetroCSIComputeAffinity(cluster)
		require.NoError(t, err)
		assert.False(t, mutated)
		assert.Equal(t, ComputeAffinityDisabled, storageClassComputeAffinity(t, cluster, "volume"))
	})

	t.Run("does not overwrite explicit non-DISABLED value", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(
			t,
			[]string{MetroFailureDomainPrefix + "metro-1"},
			"",
			map[string]map[string]string{
				"volume": {ComputeAffinityParameter: "ENABLED"},
			},
		)

		mutated, err := DefaultMetroCSIComputeAffinity(cluster)
		require.NoError(t, err)
		assert.False(t, mutated)
		assert.Equal(t, "ENABLED", storageClassComputeAffinity(t, cluster, "volume"))
	})

	t.Run("treats empty computeAffinity as unset", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(
			t,
			[]string{MetroFailureDomainPrefix + "metro-1"},
			"",
			map[string]map[string]string{
				"volume": {ComputeAffinityParameter: ""},
			},
		)

		mutated, err := DefaultMetroCSIComputeAffinity(cluster)
		require.NoError(t, err)
		assert.True(t, mutated)
		assert.Equal(t, ComputeAffinityDisabled, storageClassComputeAffinity(t, cluster, "volume"))
	})

	t.Run("no-op for non-metro cluster", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(t, []string{"pe-fd-1"}, "", nil)

		mutated, err := DefaultMetroCSIComputeAffinity(cluster)
		require.NoError(t, err)
		assert.False(t, mutated)
		assert.Empty(t, storageClassComputeAffinity(t, cluster, "volume"))
	})

	t.Run("no-op when CSI is not configured", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(t, []string{MetroSiteFailureDomainPrefix + "site-1"}, "", nil)
		cfg := mustUnmarshalClusterConfig(t, cluster)
		cfg.Addons = nil
		mustWriteClusterConfig(t, cluster, cfg)

		mutated, err := DefaultMetroCSIComputeAffinity(cluster)
		require.NoError(t, err)
		assert.False(t, mutated)
	})

	t.Run("detects metro from worker failure domain", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(t, nil, MetroFailureDomainPrefix+"metro-1", nil)

		mutated, err := DefaultMetroCSIComputeAffinity(cluster)
		require.NoError(t, err)
		assert.True(t, mutated)
		assert.Equal(t, ComputeAffinityDisabled, storageClassComputeAffinity(t, cluster, "volume"))
	})
}

func TestValidateMetroCSIComputeAffinity(t *testing.T) {
	t.Parallel()

	t.Run("rejects non-DISABLED computeAffinity on metro", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(
			t,
			[]string{MetroFailureDomainPrefix + "metro-1"},
			"",
			map[string]map[string]string{
				"volume": {ComputeAffinityParameter: "ENABLED"},
			},
		)

		err := ValidateMetroCSIComputeAffinity(cluster)
		require.ErrorContains(t, err, "not supported for the metro cluster")
		require.ErrorContains(t, err, "ENABLED")
	})

	t.Run("allows DISABLED on metro", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(
			t,
			[]string{MetroFailureDomainPrefix + "metro-1"},
			"",
			map[string]map[string]string{
				"volume": {ComputeAffinityParameter: ComputeAffinityDisabled},
			},
		)

		assert.NoError(t, ValidateMetroCSIComputeAffinity(cluster))
	})

	t.Run("allows omitted computeAffinity on metro", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(t, []string{MetroFailureDomainPrefix + "metro-1"}, "", nil)

		assert.NoError(t, ValidateMetroCSIComputeAffinity(cluster))
	})

	t.Run("allows non-DISABLED on non-metro", func(t *testing.T) {
		t.Parallel()
		cluster := clusterWithNutanixCSI(
			t,
			[]string{"pe-fd-1"},
			"",
			map[string]map[string]string{
				"volume": {ComputeAffinityParameter: "ENABLED"},
			},
		)

		assert.NoError(t, ValidateMetroCSIComputeAffinity(cluster))
	})
}

func clusterWithNutanixCSI(
	t *testing.T,
	cpFailureDomains []string,
	workerFailureDomain string,
	storageClassParameters map[string]map[string]string,
) *clusterv1.Cluster {
	t.Helper()

	if storageClassParameters == nil {
		storageClassParameters = map[string]map[string]string{
			"volume": {"storageContainer": "default-ctr"},
		}
	}

	configs := make(map[string]v1alpha1.StorageClassConfig, len(storageClassParameters))
	for name, params := range storageClassParameters {
		configs[name] = v1alpha1.StorageClassConfig{Parameters: params}
	}

	cfg := &variables.ClusterConfigSpec{
		Addons: &variables.Addons{
			CSI: &variables.CSI{
				Providers: map[string]v1alpha1.CSIProvider{
					v1alpha1.CSIProviderNutanix: {
						StorageClassConfigs: configs,
					},
				},
			},
		},
	}
	if len(cpFailureDomains) > 0 {
		cfg.ControlPlane = &variables.ControlPlaneSpec{
			Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
				FailureDomains: cpFailureDomains,
			},
		}
	}

	cluster := clusterWithConfigAndWorkers(cfg, workerFailureDomain)
	return cluster
}

func storageClassComputeAffinity(t *testing.T, cluster *clusterv1.Cluster, scName string) string {
	t.Helper()
	cfg := mustUnmarshalClusterConfig(t, cluster)
	provider, ok := cfg.Addons.CSI.Providers[v1alpha1.CSIProviderNutanix]
	require.True(t, ok)
	sc, ok := provider.StorageClassConfigs[scName]
	require.True(t, ok)
	if sc.Parameters == nil {
		return ""
	}
	return sc.Parameters[ComputeAffinityParameter]
}

func mustUnmarshalClusterConfig(t *testing.T, cluster *clusterv1.Cluster) *variables.ClusterConfigSpec {
	t.Helper()
	cfg, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	return cfg
}

func mustWriteClusterConfig(t *testing.T, cluster *clusterv1.Cluster, cfg *variables.ClusterConfigSpec) {
	t.Helper()
	variable, err := variables.MarshalToClusterVariable(v1alpha1.ClusterConfigVariableName, cfg)
	require.NoError(t, err)
	cluster.Spec.Topology.Variables = variables.UpdateClusterVariable(variable, cluster.Spec.Topology.Variables)
}
