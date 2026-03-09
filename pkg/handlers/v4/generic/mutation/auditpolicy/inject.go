// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
	"context"
	_ "embed"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
)

type auditPolicyPatchHandler struct{}

//go:embed embedded/apiserver-audit-policy.yaml
var auditPolicy string

const auditPolicyPath = "/etc/kubernetes/audit-policy.yaml"

func NewPatch() *auditPolicyPatchHandler {
	return &auditPolicyPatchHandler{}
}

func (h *auditPolicyPatchHandler) Mutate(
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

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding files and updating API server extra args in kubeadm config spec")

			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				bootstrapv1.File{
					Path:        auditPolicyPath,
					Permissions: "0600",
					Content:     auditPolicy,
				},
			)

			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer
			extraArgsMap := make(map[string]bool)
			for _, arg := range apiServer.ExtraArgs {
				extraArgsMap[arg.Name] = true
			}
			auditArgs := []bootstrapv1.Arg{
				{Name: "audit-log-path", Value: ptr.To("/var/log/audit/kube-apiserver-audit.log")},
				{Name: "audit-log-maxage", Value: ptr.To("30")},
				{Name: "audit-log-maxbackup", Value: ptr.To("10")},
				{Name: "audit-log-maxsize", Value: ptr.To("100")},
				{Name: "audit-policy-file", Value: ptr.To(auditPolicyPath)},
			}
			for _, arg := range auditArgs {
				if !extraArgsMap[arg.Name] {
					apiServer.ExtraArgs = append(apiServer.ExtraArgs, arg)
					extraArgsMap[arg.Name] = true
				}
			}

			if apiServer.ExtraVolumes == nil {
				apiServer.ExtraVolumes = make([]bootstrapv1.HostPathMount, 0, 2)
			}

			apiServer.ExtraVolumes = append(
				apiServer.ExtraVolumes,
				bootstrapv1.HostPathMount{
					Name:      "audit-policy",
					HostPath:  auditPolicyPath,
					MountPath: auditPolicyPath,
					ReadOnly:  ptr.To(true),
					PathType:  corev1.HostPathFile,
				},
				bootstrapv1.HostPathMount{
					Name:      "audit-logs",
					HostPath:  "/var/log/kubernetes/audit",
					MountPath: "/var/log/audit/",
					PathType:  corev1.HostPathDirectoryOrCreate,
				},
			)

			return nil
		},
	)
}
