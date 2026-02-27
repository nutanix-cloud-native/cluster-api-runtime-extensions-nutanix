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
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func renderAndDeserialize(
	t *testing.T,
	cfg *v1alpha1.KubeletConfiguration,
) kubeletconfigv1beta1.KubeletConfiguration {
	t.Helper()
	f, err := renderKubeletConfigPatch(cfg)
	require.NoError(t, err)

	var kubeletCfg kubeletconfigv1beta1.KubeletConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(f.Content), &kubeletCfg))
	return kubeletCfg
}

func TestRenderKubeletConfigPatch_MaxPods(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		MaxPods: ptr.To(int32(110)),
	})
	assert.Equal(t, int32(110), kubeletCfg.MaxPods)
}

func TestRenderKubeletConfigPatch_SystemReserved(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		SystemReserved: map[string]resource.Quantity{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("200Mi"),
		},
	})
	assert.Equal(t, "100m", kubeletCfg.SystemReserved["cpu"])
	assert.Equal(t, "200Mi", kubeletCfg.SystemReserved["memory"])
}

func TestRenderKubeletConfigPatch_KubeReserved(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		KubeReserved: map[string]resource.Quantity{
			"cpu": resource.MustParse("500m"),
		},
	})
	assert.Equal(t, "500m", kubeletCfg.KubeReserved["cpu"])
}

func TestRenderKubeletConfigPatch_EvictionHard(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		EvictionHard: map[string]string{
			"memory.available": "100Mi",
		},
	})
	assert.Equal(t, "100Mi", kubeletCfg.EvictionHard["memory.available"])
}

func TestRenderKubeletConfigPatch_EvictionSoft(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		EvictionSoft: map[string]string{
			"memory.available": "200Mi",
		},
	})
	assert.Equal(t, "200Mi", kubeletCfg.EvictionSoft["memory.available"])
}

func TestRenderKubeletConfigPatch_EvictionSoftGracePeriod(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		EvictionSoft: map[string]string{"memory.available": "200Mi"},
		EvictionSoftGracePeriod: map[string]metav1.Duration{
			"memory.available": {Duration: 30 * time.Second},
		},
	})
	assert.Equal(t, "30s", kubeletCfg.EvictionSoftGracePeriod["memory.available"])
}

func TestRenderKubeletConfigPatch_ProtectKernelDefaults(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		ProtectKernelDefaults: ptr.To(true),
	})
	assert.True(t, kubeletCfg.ProtectKernelDefaults)
}

func TestRenderKubeletConfigPatch_TopologyManagerPolicy(t *testing.T) {
	policy := v1alpha1.TopologyManagerPolicySingleNUMANode
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		TopologyManagerPolicy: &policy,
	})
	assert.Equal(t, "single-numa-node", kubeletCfg.TopologyManagerPolicy)
}

func TestRenderKubeletConfigPatch_CPUManagerPolicy(t *testing.T) {
	policy := v1alpha1.CPUManagerPolicyStatic
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		CPUManagerPolicy: &policy,
	})
	assert.Equal(t, "static", kubeletCfg.CPUManagerPolicy)
}

func TestRenderKubeletConfigPatch_MemoryManagerPolicy(t *testing.T) {
	policy := v1alpha1.MemoryManagerPolicyStatic
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		MemoryManagerPolicy: &policy,
	})
	assert.Equal(t, "Static", kubeletCfg.MemoryManagerPolicy)
}

func TestRenderKubeletConfigPatch_PodPidsLimit(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		PodPidsLimit: ptr.To(int64(4096)),
	})
	require.NotNil(t, kubeletCfg.PodPidsLimit)
	assert.Equal(t, int64(4096), *kubeletCfg.PodPidsLimit)
}

func TestRenderKubeletConfigPatch_ContainerLogMaxSize(t *testing.T) {
	qty := resource.MustParse("50Mi")
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		ContainerLogMaxSize: &qty,
	})
	assert.Equal(t, "50Mi", kubeletCfg.ContainerLogMaxSize)
}

func TestRenderKubeletConfigPatch_ContainerLogMaxFiles(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		ContainerLogMaxFiles: ptr.To(int32(10)),
	})
	require.NotNil(t, kubeletCfg.ContainerLogMaxFiles)
	assert.Equal(t, int32(10), *kubeletCfg.ContainerLogMaxFiles)
}

func TestRenderKubeletConfigPatch_ImageGCHighThresholdPercent(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		ImageGCHighThresholdPercent: ptr.To(int32(90)),
	})
	require.NotNil(t, kubeletCfg.ImageGCHighThresholdPercent)
	assert.Equal(t, int32(90), *kubeletCfg.ImageGCHighThresholdPercent)
}

func TestRenderKubeletConfigPatch_ImageGCLowThresholdPercent(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		ImageGCLowThresholdPercent: ptr.To(int32(70)),
	})
	require.NotNil(t, kubeletCfg.ImageGCLowThresholdPercent)
	assert.Equal(t, int32(70), *kubeletCfg.ImageGCLowThresholdPercent)
}

func TestRenderKubeletConfigPatch_MaxParallelImagePulls(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		MaxParallelImagePulls: ptr.To(int32(10)),
	})
	require.NotNil(t, kubeletCfg.SerializeImagePulls)
	assert.False(t, *kubeletCfg.SerializeImagePulls)
	require.NotNil(t, kubeletCfg.MaxParallelImagePulls)
	assert.Equal(t, int32(10), *kubeletCfg.MaxParallelImagePulls)
}

func TestRenderKubeletConfigPatch_ShutdownGracePeriod(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		ShutdownGracePeriod: &metav1.Duration{Duration: 60 * time.Second},
	})
	assert.Equal(t, metav1.Duration{Duration: 60 * time.Second}, kubeletCfg.ShutdownGracePeriod)
}

func TestRenderKubeletConfigPatch_ShutdownGracePeriodCriticalPods(t *testing.T) {
	kubeletCfg := renderAndDeserialize(t, &v1alpha1.KubeletConfiguration{
		ShutdownGracePeriodCriticalPods: &metav1.Duration{Duration: 10 * time.Second},
	})
	assert.Equal(t,
		metav1.Duration{Duration: 10 * time.Second},
		kubeletCfg.ShutdownGracePeriodCriticalPods,
	)
}

func TestIsKubeletConfigEmpty(t *testing.T) {
	assert.True(t, isKubeletConfigEmpty(nil))

	empty := &v1alpha1.KubeletConfiguration{}
	assert.True(t, isKubeletConfigEmpty(empty))

	withField := &v1alpha1.KubeletConfiguration{MaxPods: ptr.To(int32(110))}
	assert.False(t, isKubeletConfigEmpty(withField))
}

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

func TestApplyDeprecatedMaxParallelImagePulls_OnlyDeprecated(t *testing.T) {
	merged := &v1alpha1.KubeletConfiguration{}
	vars := map[string]apiextensionsv1.JSON{
		v1alpha1.ClusterConfigVariableName: {
			Raw: []byte(`{"maxParallelImagePullsPerNode": 4}`),
		},
	}

	result, err := applyDeprecatedMaxParallelImagePulls(
		merged, vars, v1alpha1.ClusterConfigVariableName,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(4), *result.MaxParallelImagePulls)
}

func TestApplyDeprecatedMaxParallelImagePulls_NewFieldWins(t *testing.T) {
	merged := &v1alpha1.KubeletConfiguration{MaxParallelImagePulls: ptr.To(int32(8))}
	vars := map[string]apiextensionsv1.JSON{
		v1alpha1.ClusterConfigVariableName: {
			Raw: []byte(`{"maxParallelImagePullsPerNode": 4}`),
		},
	}

	result, err := applyDeprecatedMaxParallelImagePulls(
		merged, vars, v1alpha1.ClusterConfigVariableName,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(8), *result.MaxParallelImagePulls)
}
