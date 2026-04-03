// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sortextraargs

import (
	"cmp"
	"context"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
)

type sortExtraArgsPatchHandler struct{}

func NewPatch() *sortExtraArgsPatchHandler {
	return &sortExtraArgsPatchHandler{}
}

func (h *sortExtraArgsPatchHandler) Mutate(
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

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).V(5).Info("sorting extra args in control plane kubeadm config spec")

			spec := &obj.Spec.Template.Spec.KubeadmConfigSpec

			sortArgs(spec.ClusterConfiguration.APIServer.ExtraArgs)
			sortArgs(spec.ClusterConfiguration.ControllerManager.ExtraArgs)
			sortArgs(spec.ClusterConfiguration.Scheduler.ExtraArgs)
			sortArgs(spec.ClusterConfiguration.Etcd.Local.ExtraArgs)
			sortArgs(spec.InitConfiguration.NodeRegistration.KubeletExtraArgs)
			sortArgs(spec.JoinConfiguration.NodeRegistration.KubeletExtraArgs)

			return nil
		}); err != nil {
		return err
	}

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.WorkersKubeadmConfigTemplateSelector(), log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).V(5).Info("sorting extra args in worker node kubeadm config template")

			sortArgs(obj.Spec.Template.Spec.JoinConfiguration.NodeRegistration.KubeletExtraArgs)

			return nil
		})
}

func sortArgs(args []bootstrapv1.Arg) {
	slices.SortFunc(args, func(a, b bootstrapv1.Arg) int {
		return cmp.Compare(a.Name, b.Name)
	})
}
