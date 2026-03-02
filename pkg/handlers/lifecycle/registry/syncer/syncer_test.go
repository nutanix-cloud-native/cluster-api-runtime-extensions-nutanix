// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package syncer

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/feature"
)

var (
	//go:embed testdata/registry-syncer-template.yaml.tmpl
	testRegistrySyncerTemplate string

	//go:embed testdata/registry-syncer-values.yaml
	expectedRegistrySyncerValues string
)

func Test_shouldApplyRegistrySyncer(t *testing.T) {
	// Pre-create a cluster so it can be used in a test case that requires the same name and namespace.
	clusterWithSameNameAndNamespace := clusterWithRegistry(t)
	tests := []struct {
		name              string
		cluster           *clusterv1.Cluster
		managementCluster *clusterv1.Cluster
		enableFeatureGate bool
		shouldApply       bool
	}{
		{
			name:              "should apply",
			cluster:           clusterWithRegistry(t),
			managementCluster: clusterWithRegistry(t),
			enableFeatureGate: true,
			shouldApply:       true,
		},
		{
			name:              "should not apply when management cluster is nil",
			cluster:           clusterWithRegistry(t),
			managementCluster: nil,
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when management cluster name and namespace matches cluster",
			cluster:           clusterWithSameNameAndNamespace,
			managementCluster: clusterWithSameNameAndNamespace,
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when feature gate is disabled",
			cluster:           clusterWithRegistry(t),
			managementCluster: clusterWithRegistry(t),
			enableFeatureGate: false,
			shouldApply:       false,
		},
		{
			name: "should not apply when cluster has skip annotation",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						carenv1.SkipSynchronizingWorkloadClusterRegistry: "true",
					},
				},
			},
			managementCluster: clusterWithRegistry(t),
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when cluster does not have registry enabled",
			cluster:           clusterWithoutRegistry(t),
			managementCluster: clusterWithRegistry(t),
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when management cluster does not have registry enabled",
			cluster:           clusterWithRegistry(t),
			managementCluster: clusterWithoutRegistry(t),
			enableFeatureGate: true,
			shouldApply:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enableFeatureGate(t, tt.enableFeatureGate)
			shouldApply, err := shouldApplyRegistrySyncer(tt.cluster, tt.managementCluster)
			require.NoError(t, err)
			assert.Equal(t, tt.shouldApply, shouldApply)
		})
	}
}

func Test_templateValues(t *testing.T) {
	result, err := templateValues(namedClusterWithRegistry(t, "test-cluster"), testRegistrySyncerTemplate)
	require.NoError(t, err)
	assert.Equal(t, expectedRegistrySyncerValues, result)
}

func clusterWithRegistry(t *testing.T) *clusterv1.Cluster {
	t.Helper()

	return namedClusterWithRegistry(t, names.SimpleNameGenerator.GenerateName("with-registry-"))
}

func namedClusterWithRegistry(t *testing.T, name string) *clusterv1.Cluster {
	t.Helper()

	clusterConfigSpec := &carenv1.DockerClusterConfigSpec{
		Addons: &carenv1.DockerAddons{
			GenericAddons: carenv1.GenericAddons{
				CNI:      &carenv1.CNI{},
				Registry: &carenv1.RegistryAddon{},
			},
		},
	}
	variable, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfigSpec)
	require.NoError(t, err)
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: clusterv1.ClusterNetwork{
				Services: clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			Topology: clusterv1.Topology{
				ClassRef:  clusterv1.ClusterClassRef{Name: "dummy-class"},
				Variables: []clusterv1.ClusterVariable{*variable},
				Version:   "v1.30.100",
			},
		},
	}
}

func clusterWithoutRegistry(t *testing.T) *clusterv1.Cluster {
	t.Helper()

	clusterConfigSpec := &carenv1.DockerClusterConfigSpec{
		Addons: &carenv1.DockerAddons{
			GenericAddons: carenv1.GenericAddons{
				CNI: &carenv1.CNI{},
			},
		},
	}
	variable, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfigSpec)
	require.NoError(t, err)
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.SimpleNameGenerator.GenerateName("without-registry-"),
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: clusterv1.ClusterNetwork{
				Services: clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			Topology: clusterv1.Topology{
				ClassRef:  clusterv1.ClusterClassRef{Name: "dummy-class"},
				Variables: []clusterv1.ClusterVariable{*variable},
			},
		},
	}
}

func enableFeatureGate(t *testing.T, value bool) {
	t.Helper()

	featuregatetesting.SetFeatureGateDuringTest(
		t,
		feature.Gates,
		feature.SynchronizeWorkloadClusterRegistry,
		value,
	)
}
