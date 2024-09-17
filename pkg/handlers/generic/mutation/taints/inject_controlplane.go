// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package taints

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

type taintsControlPlanePatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewControlPlanePatch() *taintsControlPlanePatchHandler {
	return newTaintsControlPlanePatchHandler(
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.ControlPlaneConfigVariableName,
		VariableName,
	)
}

func newTaintsControlPlanePatchHandler(
	variableName string,
	variableFieldPath ...string,
) *taintsControlPlanePatchHandler {
	return &taintsControlPlanePatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *taintsControlPlanePatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ ctrlclient.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	taintsVar, err := variables.Get[[]v1alpha1.Taint](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Taints variable for worker not defined")
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
		taintsVar,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding taints to worker node kubeadm config template")
			if obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration != nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration = &bootstrapv1.InitConfiguration{}
			}
			if obj.Spec.Template.Spec.KubeadmConfigSpec.JoinConfiguration != nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.JoinConfiguration = &bootstrapv1.JoinConfiguration{}
			}
			coreTaints := toCoreTaints(taintsVar)

			obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration.NodeRegistration.Taints = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration.NodeRegistration.Taints,
				coreTaints...,
			)
			obj.Spec.Template.Spec.KubeadmConfigSpec.JoinConfiguration.NodeRegistration.Taints = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.JoinConfiguration.NodeRegistration.Taints,
				coreTaints...,
			)

			return nil
		})
}
