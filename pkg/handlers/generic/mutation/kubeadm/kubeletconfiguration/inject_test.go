// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// Template rendering tests: each of the 17 fields individually.
func TestRenderKubeletConfigPatch_MaxPods(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(110))}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "maxPods: 110")
}

func TestRenderKubeletConfigPatch_SystemReserved(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{
		SystemReserved: map[string]resource.Quantity{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("200Mi"),
		},
	}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "systemReserved:")
	assert.Contains(t, f.Content, "cpu:")
	assert.Contains(t, f.Content, "memory:")
}

func TestRenderKubeletConfigPatch_KubeReserved(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{
		KubeReserved: map[string]resource.Quantity{
			"cpu": resource.MustParse("500m"),
		},
	}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "kubeReserved:")
	assert.Contains(t, f.Content, "cpu:")
}

func TestRenderKubeletConfigPatch_EvictionHard(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{
		EvictionHard: map[string]string{
			"memory.available": "100Mi",
		},
	}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "evictionHard:")
	assert.Contains(t, f.Content, "memory.available:")
}

func TestRenderKubeletConfigPatch_EvictionSoft(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{
		EvictionSoft: map[string]string{
			"memory.available": "200Mi",
		},
	}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "evictionSoft:")
	assert.Contains(t, f.Content, "memory.available:")
}

func TestRenderKubeletConfigPatch_EvictionSoftGracePeriod(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{
		EvictionSoft: map[string]string{"memory.available": "200Mi"},
		EvictionSoftGracePeriod: map[string]metav1.Duration{
			"memory.available": {Duration: 30 * time.Second},
		},
	}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "evictionSoftGracePeriod:")
	assert.Contains(t, f.Content, "30s")
}

func TestRenderKubeletConfigPatch_ProtectKernelDefaults(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{ProtectKernelDefaults: ptr.To(true)}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "protectKernelDefaults: true")
}

func TestRenderKubeletConfigPatch_TopologyManagerPolicy(t *testing.T) {
	policy := v1alpha1.TopologyManagerPolicySingleNUMANode
	cfg := &v1alpha1.KubeletConfiguration{TopologyManagerPolicy: &policy}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "topologyManagerPolicy: single-numa-node")
}

func TestRenderKubeletConfigPatch_CPUManagerPolicy(t *testing.T) {
	policy := v1alpha1.CPUManagerPolicyStatic
	cfg := &v1alpha1.KubeletConfiguration{CPUManagerPolicy: &policy}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "cpuManagerPolicy: static")
}

func TestRenderKubeletConfigPatch_MemoryManagerPolicy(t *testing.T) {
	policy := v1alpha1.MemoryManagerPolicyStatic
	cfg := &v1alpha1.KubeletConfiguration{MemoryManagerPolicy: &policy}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "memoryManagerPolicy: Static")
}

func TestRenderKubeletConfigPatch_PodPidsLimit(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{PodPidsLimit: ptr.To(int64(4096))}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "podPidsLimit: 4096")
}

func TestRenderKubeletConfigPatch_ContainerLogMaxSize(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{ContainerLogMaxSize: ptr.To("50Mi")}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "containerLogMaxSize: 50Mi")
}

func TestRenderKubeletConfigPatch_ContainerLogMaxFiles(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{ContainerLogMaxFiles: ptr.To(int32(10))}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "containerLogMaxFiles: 10")
}

func TestRenderKubeletConfigPatch_ImageGCHighThresholdPercent(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{ImageGCHighThresholdPercent: ptr.To(int32(90))}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "imageGCHighThresholdPercent: 90")
}

func TestRenderKubeletConfigPatch_ImageGCLowThresholdPercent(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{ImageGCLowThresholdPercent: ptr.To(int32(70))}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "imageGCLowThresholdPercent: 70")
}

func TestRenderKubeletConfigPatch_MaxParallelImagePulls(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{MaxParallelImagePulls: ptr.To(int32(10))}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "serializeImagePulls: false")
	assert.Contains(t, f.Content, "maxParallelImagePulls: 10")
}

func TestRenderKubeletConfigPatch_ShutdownGracePeriod(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{
		ShutdownGracePeriod: &metav1.Duration{Duration: 60 * time.Second},
	}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "shutdownGracePeriod:")
}

func TestRenderKubeletConfigPatch_ShutdownGracePeriodCriticalPods(t *testing.T) {
	cfg := &v1alpha1.KubeletConfiguration{
		ShutdownGracePeriodCriticalPods: &metav1.Duration{Duration: 10 * time.Second},
	}
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)
	assert.Contains(t, f.Content, "shutdownGracePeriodCriticalPods:")
}

// Empty config test.
func TestIsKubeletConfigEmpty(t *testing.T) {
	assert.True(t, isKubeletConfigEmpty(nil))

	empty := &v1alpha1.KubeletConfiguration{}
	assert.True(t, isKubeletConfigEmpty(empty))

	withField := &v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(110))}
	assert.False(t, isKubeletConfigEmpty(withField))
}

// Merge tests.
func TestMergeKubeletConfig_ClusterOnly(t *testing.T) {
	cluster := &v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(110))}
	node := (*v1alpha1.KubeletConfiguration)(nil)

	merged := mergeKubeletConfig(cluster, node)
	require.NotNil(t, merged)
	assert.Equal(t, int32(110), *merged.MaxPods)
}

func TestMergeKubeletConfig_NodeOnly(t *testing.T) {
	cluster := (*v1alpha1.KubeletConfiguration)(nil)
	node := &v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(50))}

	merged := mergeKubeletConfig(cluster, node)
	require.NotNil(t, merged)
	assert.Equal(t, int32(50), *merged.MaxPods)
}

func TestMergeKubeletConfig_UnionDifferentFields(t *testing.T) {
	cluster := &v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(110))}
	node := &v1alpha1.KubeletConfiguration{ProtectKernelDefaults: ptr.To(true)}

	merged := mergeKubeletConfig(cluster, node)
	require.NotNil(t, merged)
	assert.Equal(t, int32(110), *merged.MaxPods)
	assert.True(t, *merged.ProtectKernelDefaults)
}

func TestMergeKubeletConfig_NodeWins(t *testing.T) {
	cluster := &v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(110))}
	node := &v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(50))}

	merged := mergeKubeletConfig(cluster, node)
	require.NotNil(t, merged)
	assert.Equal(t, int32(50), *merged.MaxPods)
}

// Deprecated field tests.
func TestApplyDeprecatedMaxParallelImagePulls_OnlyDeprecated(t *testing.T) {
	merged := &v1alpha1.KubeletConfiguration{}
	vars := map[string]apiextensionsv1.JSON{
		v1alpha1.ClusterConfigVariableName: {Raw: []byte(`{"maxParallelImagePullsPerNode": 4}`)},
	}

	result, err := applyDeprecatedMaxParallelImagePulls(merged, vars, v1alpha1.ClusterConfigVariableName)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(4), *result.MaxParallelImagePulls)
}

func TestApplyDeprecatedMaxParallelImagePulls_NewFieldWins(t *testing.T) {
	merged := &v1alpha1.KubeletConfiguration{MaxParallelImagePulls: ptr.To(int32(8))}
	vars := map[string]apiextensionsv1.JSON{
		v1alpha1.ClusterConfigVariableName: {Raw: []byte(`{"maxParallelImagePullsPerNode": 4}`)},
	}

	result, err := applyDeprecatedMaxParallelImagePulls(merged, vars, v1alpha1.ClusterConfigVariableName)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(8), *result.MaxParallelImagePulls)
}
