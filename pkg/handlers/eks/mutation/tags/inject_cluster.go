// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tags

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	eksv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "additionalTags"
)

type eksTagsClusterPatchHandler struct {
	metaVariableName  string
	variableFieldPath []string
	patchSelector     clusterv1.PatchSelector
}

func newEKSClusterPatchHandler(
	metaVariableName string,
	variableFieldPath []string,
	patchSelector clusterv1.PatchSelector,
) *eksTagsClusterPatchHandler {
	return &eksTagsClusterPatchHandler{
		metaVariableName:  metaVariableName,
		variableFieldPath: variableFieldPath,
		patchSelector:     patchSelector,
	}
}

func (h *eksTagsClusterPatchHandler) Mutate(
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

	additionalTagsVar, err := variables.Get[capav1.Tags](
		vars,
		h.metaVariableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("EKS additionalTags variable for control plane not defined")
			return nil
		}
		return err
	}

	log = log.WithValues(
		"variableName",
		h.metaVariableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		additionalTagsVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		h.patchSelector,
		log,
		func(obj *eksv1.AWSManagedControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting additionalTags in AWSManagedControlPlaneTemplate spec")

			obj.Spec.Template.Spec.AdditionalTags = additionalTagsVar

			return nil
		},
	)
}

func NewClusterPatch() *eksTagsClusterPatchHandler {
	return newEKSClusterPatchHandler(
		v1alpha1.ClusterConfigVariableName,
		[]string{
			v1alpha1.EKSVariableName,
			VariableName,
		},
		clusterv1.PatchSelector{
			APIVersion: eksv1.GroupVersion.String(),
			Kind:       "AWSManagedControlPlaneTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				ControlPlane: true,
			},
		},
	)
}
