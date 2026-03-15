// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

type kubeletConfigurationWorkerPatchHandler struct {
	clusterVariableName      string
	clusterVariableFieldPath []string
	workerVariableName       string
	workerVariableFieldPath  []string
}

// NewWorkerPatch returns a handler that merges cluster-level and worker
// kubeletConfiguration and applies the patch to KubeadmConfigTemplate.
func NewWorkerPatch() *kubeletConfigurationWorkerPatchHandler {
	return &kubeletConfigurationWorkerPatchHandler{
		clusterVariableName:      v1alpha1.ClusterConfigVariableName,
		clusterVariableFieldPath: []string{VariableName},
		workerVariableName:       v1alpha1.WorkerConfigVariableName,
		workerVariableFieldPath:  []string{VariableName},
	}
}

func (h *kubeletConfigurationWorkerPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	var clusterCfg *v1alpha1.KubeletConfiguration
	if cfg, err := variables.Get[v1alpha1.KubeletConfiguration](
		vars,
		h.clusterVariableName,
		h.clusterVariableFieldPath...,
	); err == nil {
		clusterCfg = &cfg
	} else if !variables.IsNotFoundError(err) {
		return err
	}

	var workerCfg *v1alpha1.KubeletConfiguration
	if cfg, err := variables.Get[v1alpha1.KubeletConfiguration](
		vars,
		h.workerVariableName,
		h.workerVariableFieldPath...,
	); err == nil {
		workerCfg = &cfg
	} else if !variables.IsNotFoundError(err) {
		return err
	}

	merged := mergeKubeletConfig(clusterCfg, workerCfg)
	merged, err := applyDeprecatedMaxParallelImagePulls(merged, vars, h.clusterVariableName)
	if err != nil {
		return err
	}
	if isKubeletConfigEmpty(merged) {
		log.V(5).Info("kubeletConfiguration is empty after merge, skipping worker mutation")
		return nil
	}

	log = log.WithValues(
		"variableName", h.clusterVariableName,
		"variableFieldPath", h.clusterVariableFieldPath,
		"workerVariableFieldPath", h.workerVariableFieldPath,
	)

	kubeletConfigPatch, err := renderKubeletConfigPatch(merged)
	if err != nil {
		return err
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.WorkersKubeadmConfigTemplateSelector(),
		log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding KubeletConfiguration patch to worker node kubeadm config template")

			obj.Spec.Template.Spec.Files = append(
				obj.Spec.Template.Spec.Files,
				*kubeletConfigPatch,
			)

			return nil
		},
	)
}
