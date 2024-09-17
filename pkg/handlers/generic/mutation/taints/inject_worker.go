// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package taints

import (
	"context"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const VariableName = "taints"

type taintsWorkerPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewWorkerPatch() *taintsWorkerPatchHandler {
	return newTaintsWorkerPatchHandler(
		v1alpha1.WorkerConfigVariableName,
		VariableName,
	)
}

func newTaintsWorkerPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *taintsWorkerPatchHandler {
	return &taintsWorkerPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *taintsWorkerPatchHandler) Mutate(
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
		obj, vars, &holderRef, selectors.WorkersKubeadmConfigTemplateSelector(), log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding taints to worker node kubeadm config template")
			if obj.Spec.Template.Spec.JoinConfiguration != nil {
				obj.Spec.Template.Spec.JoinConfiguration = &bootstrapv1.JoinConfiguration{}
			}
			obj.Spec.Template.Spec.JoinConfiguration.NodeRegistration.Taints = append(
				obj.Spec.Template.Spec.JoinConfiguration.NodeRegistration.Taints,
				toCoreTaints(taintsVar)...,
			)
			return nil
		})
}

func toCoreTaints(taints []v1alpha1.Taint) []v1.Taint {
	return lo.Map(taints, func(t v1alpha1.Taint, _ int) v1.Taint {
		return v1.Taint{
			Key:    t.Key,
			Effect: v1.TaintEffect(t.Effect),
			Value:  t.Value,
		}
	})
}
