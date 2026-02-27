// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

type kubeletConfigurationControlPlanePatchHandler struct {
	clusterVariableName      string
	clusterVariableFieldPath []string
	cpVariableName           string
	cpVariableFieldPath      []string
}

// NewControlPlanePatch returns a handler that merges cluster-level and control-plane
// kubeletConfiguration and applies the patch to KubeadmControlPlaneTemplate.
func NewControlPlanePatch() *kubeletConfigurationControlPlanePatchHandler {
	return &kubeletConfigurationControlPlanePatchHandler{
		clusterVariableName:      v1alpha1.ClusterConfigVariableName,
		clusterVariableFieldPath: []string{VariableName},
		cpVariableName:           v1alpha1.ClusterConfigVariableName,
		cpVariableFieldPath:      []string{v1alpha1.ControlPlaneConfigVariableName, VariableName},
	}
}

func (h *kubeletConfigurationControlPlanePatchHandler) Mutate(
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

	var cpCfg *v1alpha1.KubeletConfiguration
	if cfg, err := variables.Get[v1alpha1.KubeletConfiguration](
		vars,
		h.cpVariableName,
		h.cpVariableFieldPath...,
	); err == nil {
		cpCfg = &cfg
	} else if !variables.IsNotFoundError(err) {
		return err
	}

	merged := mergeKubeletConfig(clusterCfg, cpCfg)
	merged, err := applyDeprecatedMaxParallelImagePulls(merged, vars, h.clusterVariableName)
	if err != nil {
		return err
	}
	if isKubeletConfigEmpty(merged) {
		log.V(5).Info("kubeletConfiguration is empty after merge, skipping control plane mutation")
		return nil
	}

	log = log.WithValues(
		"variableName", h.clusterVariableName,
		"variableFieldPath", h.clusterVariableFieldPath,
		"cpVariableFieldPath", h.cpVariableFieldPath,
	)

	kubeletConfigPatch, err := renderKubeletConfigPatch(merged)
	if err != nil {
		return err
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding KubeletConfiguration patch to control plane kubeadm config spec")

			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				*kubeletConfigPatch,
			)

			return nil
		},
	)
}
