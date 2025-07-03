// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanefailuredomains

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "failureDomains"
)

type nutanixControlPlaneFailureDomains struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *nutanixControlPlaneFailureDomains {
	return newNutanixControlPlaneFailureDomains(
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.ControlPlaneConfigVariableName,
		v1alpha1.NutanixVariableName,
		VariableName,
	)
}

func newNutanixControlPlaneFailureDomains(
	variableName string,
	variableFieldPath ...string,
) *nutanixControlPlaneFailureDomains {
	return &nutanixControlPlaneFailureDomains{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *nutanixControlPlaneFailureDomains) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
		"variableName", h.variableName,
		"variableFieldPath", h.variableFieldPath,
	)

	controlPlaneFDsVar, err := variables.Get[[]string](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("ControlPlane nutanix failureDomains variable not defined", "error", err.Error())
			return nil
		}
		log.V(5).Error(err, "failed to get controlPlane nutanix failureDomains variable")
		return err
	}

	log = log.WithValues("variableValue", controlPlaneFDsVar)

	if len(controlPlaneFDsVar) == 0 {
		log.V(5).Info("ControlPlane nutanix failureDomains variable is empty")
		return nil
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster(capxv1.GroupVersion.Version, "NutanixClusterTemplate"),
		log,
		func(obj *capxv1.NutanixClusterTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting controlPlaneFailureDomains in NutanixCluster spec")

			fdRefs := []corev1.LocalObjectReference{}
			for _, fd := range controlPlaneFDsVar {
				fdRefs = append(fdRefs, corev1.LocalObjectReference{Name: fd})
			}

			// set controlPlaneFailureDomains in NutanixCluster spec
			obj.Spec.Template.Spec.ControlPlaneFailureDomains = fdRefs

			return nil
		},
	)
}
