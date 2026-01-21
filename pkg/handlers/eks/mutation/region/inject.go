// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package region

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	eksv1 "sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "region"
)

type eksRegionPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *eksRegionPatchHandler {
	return newEKSRegionPatchHandler(
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.EKSVariableName,
		VariableName,
	)
}

func newEKSRegionPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *eksRegionPatchHandler {
	return &eksRegionPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *eksRegionPatchHandler) Mutate(
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

	regionVar, err := variables.Get[v1alpha1.Region](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("EKS region variable not defined")
			return nil
		}
		return err
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		regionVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		clusterv1.PatchSelector{
			APIVersion: eksv1.GroupVersion.String(),
			Kind:       "AWSManagedControlPlaneTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				ControlPlane: true,
			},
		},
		log,
		func(obj *eksv1.AWSManagedControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting region in AWSManagedControlPlaneTemplate spec")

			obj.Spec.Template.Spec.Region = string(regionVar)

			return nil
		},
	)
}
