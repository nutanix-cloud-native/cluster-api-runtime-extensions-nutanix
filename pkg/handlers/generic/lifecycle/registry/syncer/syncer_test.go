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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

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
	clusterWithSameNameAndNamespace := newTestCluster(t).withRegistry().build()
	tests := []struct {
		name              string
		cluster           *clusterv1.Cluster
		managementCluster *clusterv1.Cluster
		enableFeatureGate bool
		shouldApply       bool
	}{
		{
			name:              "should apply",
			cluster:           newTestCluster(t).withRegistry().build(),
			managementCluster: newTestCluster(t).withRegistry().build(),
			enableFeatureGate: true,
			shouldApply:       true,
		},
		{
			name:              "should not apply when management cluster is nil",
			cluster:           newTestCluster(t).withRegistry().build(),
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
			cluster:           newTestCluster(t).withRegistry().build(),
			managementCluster: newTestCluster(t).withRegistry().build(),
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
			managementCluster: newTestCluster(t).withRegistry().build(),
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when cluster does not have registry enabled",
			cluster:           newTestCluster(t).build(),
			managementCluster: newTestCluster(t).withRegistry().build(),
			enableFeatureGate: true,
			shouldApply:       false,
		},
		{
			name:              "should not apply when management cluster does not have registry enabled",
			cluster:           newTestCluster(t).withRegistry().build(),
			managementCluster: newTestCluster(t).build(),
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
	result, err := templateValues(
		newTestCluster(t).withRegistry().withName("test-cluster").build(),
		testRegistrySyncerTemplate,
	)
	require.NoError(t, err)
	assert.Equal(t, expectedRegistrySyncerValues, result)
}

func Test_templateValues_withDirtyVersion(t *testing.T) {
	result, err := templateValues(
		newTestCluster(t).withRegistry().withName("test-cluster").withVersion("v1.30.100+build.1").build(),
		testRegistrySyncerTemplate,
	)
	require.NoError(t, err)
	assert.Equal(t, expectedRegistrySyncerValues, result)
}

type testCluster struct {
	t *testing.T

	name           string
	enableRegistry bool
	version        string
}

func newTestCluster(t *testing.T) *testCluster {
	t.Helper()

	return &testCluster{
		t:       t,
		name:    names.SimpleNameGenerator.GenerateName("test-cluster-"),
		version: "v1.30.100",
	}
}

func (t *testCluster) withName(name string) *testCluster {
	t.name = name
	return t
}

func (t *testCluster) withRegistry() *testCluster {
	t.name = names.SimpleNameGenerator.GenerateName("with-registry-")
	t.enableRegistry = true
	return t
}

func (t *testCluster) withVersion(version string) *testCluster {
	t.version = version
	return t
}

func (t *testCluster) build() *clusterv1.Cluster {
	clusterConfigSpec := &carenv1.DockerClusterConfigSpec{
		Addons: &carenv1.DockerAddons{
			GenericAddons: carenv1.GenericAddons{
				CNI: &carenv1.CNI{},
			},
		},
	}
	if t.enableRegistry {
		clusterConfigSpec.Addons.Registry = &carenv1.RegistryAddon{}
	}

	variable, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfigSpec)
	require.NoError(t.t, err)

	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.name,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			Topology: &clusterv1.Topology{
				Variables: []clusterv1.ClusterVariable{
					*variable,
				},
				Version: t.version,
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
