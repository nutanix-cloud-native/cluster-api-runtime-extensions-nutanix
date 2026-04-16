// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	"bytes"
	_ "embed"
	"fmt"
	"slices"
	"text/template"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "kubeletConfiguration"

	kubeletConfigurationPatchFilePath = "/etc/kubernetes/patches/kubeletconfiguration99+strategic.json"
)

var (
	//go:embed embedded/kubeletconfigpatch.yaml.tmpl
	kubeletConfigPatchYAML []byte

	kubeletConfigPatchTemplate = template.Must(template.New("kubeletConfigPatch").Parse(string(kubeletConfigPatchYAML)))
)

// kubeletConfigTemplateInput holds template-friendly values (dereferenced, stringified).
type kubeletConfigTemplateInput struct {
	MaxPods                         any
	SystemReserved                  map[string]string
	KubeReserved                    map[string]string
	EvictionHard                    map[string]string
	EvictionSoft                    map[string]string
	EvictionSoftGracePeriod         map[string]string
	ProtectKernelDefaults           any
	TopologyManagerPolicy           any
	CPUManagerPolicy                any
	MemoryManagerPolicy             any
	PodPidsLimit                    any
	ContainerLogMaxSize             any
	ContainerLogMaxFiles            any
	ImageGCHighThresholdPercent     any
	ImageGCLowThresholdPercent      any
	MaxParallelImagePulls           any
	ShutdownGracePeriod             any
	ShutdownGracePeriodCriticalPods any
	SeccompDefault                  any
	EnforceNodeAllocatable          []string
	SystemReservedCgroup            string
	KubeReservedCgroup              string
}

// toTemplateInput converts v1alpha1.KubeletConfiguration to template-friendly struct.
func toTemplateInput(cfg *v1alpha1.KubeletConfiguration) *kubeletConfigTemplateInput {
	if cfg == nil {
		return &kubeletConfigTemplateInput{}
	}

	in := &kubeletConfigTemplateInput{
		MaxPods:                     cfg.MaxPods,
		ProtectKernelDefaults:       cfg.ProtectKernelDefaults,
		TopologyManagerPolicy:       cfg.TopologyManagerPolicy,
		CPUManagerPolicy:            cfg.CPUManagerPolicy,
		MemoryManagerPolicy:         cfg.MemoryManagerPolicy,
		PodPidsLimit:                cfg.PodPidsLimit,
		ContainerLogMaxFiles:        cfg.ContainerLogMaxFiles,
		ImageGCHighThresholdPercent: cfg.ImageGCHighThresholdPercent,
		ImageGCLowThresholdPercent:  cfg.ImageGCLowThresholdPercent,
		MaxParallelImagePulls:       cfg.MaxParallelImagePulls,
		SeccompDefault:              cfg.SeccompDefault,
	}

	if cfg.ContainerLogMaxSize != nil {
		in.ContainerLogMaxSize = cfg.ContainerLogMaxSize.String()
	}
	if cfg.ShutdownGracePeriod != nil {
		in.ShutdownGracePeriod = cfg.ShutdownGracePeriod.Duration.String()
	}
	if cfg.ShutdownGracePeriodCriticalPods != nil {
		in.ShutdownGracePeriodCriticalPods = cfg.ShutdownGracePeriodCriticalPods.Duration.String()
	}

	if len(cfg.SystemReserved) > 0 {
		in.SystemReserved = make(map[string]string, len(cfg.SystemReserved))
		for k, v := range cfg.SystemReserved {
			in.SystemReserved[k] = v.String()
		}
	}
	if len(cfg.KubeReserved) > 0 {
		in.KubeReserved = make(map[string]string, len(cfg.KubeReserved))
		for k, v := range cfg.KubeReserved {
			in.KubeReserved[k] = v.String()
		}
	}
	if len(cfg.EvictionHard) > 0 {
		in.EvictionHard = cfg.EvictionHard
	}
	if len(cfg.EvictionSoft) > 0 {
		in.EvictionSoft = cfg.EvictionSoft
	}
	if len(cfg.EvictionSoftGracePeriod) > 0 {
		in.EvictionSoftGracePeriod = make(map[string]string, len(cfg.EvictionSoftGracePeriod))
		for k, v := range cfg.EvictionSoftGracePeriod {
			in.EvictionSoftGracePeriod[k] = v.Duration.String()
		}
	}

	if len(cfg.EnforceNodeAllocatable) > 0 {
		sorted := make([]string, len(cfg.EnforceNodeAllocatable))
		for i, v := range cfg.EnforceNodeAllocatable {
			sorted[i] = string(v)
		}
		slices.Sort(sorted)
		in.EnforceNodeAllocatable = sorted
		if slices.Contains(sorted, string(v1alpha1.EnforceNodeAllocatableSystemReserved)) ||
			slices.Contains(sorted, string(v1alpha1.EnforceNodeAllocatableSystemReservedCompressible)) {
			in.SystemReservedCgroup = "/system.slice"
		}
		if slices.Contains(sorted, string(v1alpha1.EnforceNodeAllocatableKubeReserved)) ||
			slices.Contains(sorted, string(v1alpha1.EnforceNodeAllocatableKubeReservedCompressible)) {
			in.KubeReservedCgroup = "/system.slice/kubelet.service"
		}
	}

	return in
}

func renderKubeletConfigPatch(cfg *v1alpha1.KubeletConfiguration) (*bootstrapv1.File, error) {
	templateInput := toTemplateInput(cfg)
	var b bytes.Buffer
	if err := kubeletConfigPatchTemplate.Execute(&b, templateInput); err != nil {
		return nil, fmt.Errorf("failed executing kubeletconfig patch template: %w", err)
	}

	return &bootstrapv1.File{
		Path:        kubeletConfigurationPatchFilePath,
		Owner:       "root:root",
		Permissions: "0644",
		Content:     b.String(),
	}, nil
}

// applyDeprecatedMaxParallelImagePulls copies the deprecated maxParallelImagePullsPerNode
// into cfg.MaxParallelImagePulls if the new field is not set. The new field takes
// precedence; if both are set, the deprecated value is ignored.
func applyDeprecatedMaxParallelImagePulls(
	cfg *v1alpha1.KubeletConfiguration,
	vars map[string]apiextensionsv1.JSON,
) (*v1alpha1.KubeletConfiguration, error) {
	deprecatedVal, err := variables.Get[int32](vars, v1alpha1.ClusterConfigVariableName, "maxParallelImagePullsPerNode")
	if err != nil {
		if variables.IsNotFoundError(err) {
			return cfg, nil
		}
		return nil, err
	}
	// New field takes precedence; skip if already set
	if cfg != nil && cfg.MaxParallelImagePulls != nil {
		return cfg, nil
	}
	if cfg == nil {
		cfg = &v1alpha1.KubeletConfiguration{}
	}
	cfg.MaxParallelImagePulls = ptr.To(deprecatedVal)
	return cfg, nil
}
