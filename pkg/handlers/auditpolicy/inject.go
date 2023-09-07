// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditpolicy

import (
	"context"
	_ "embed"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "AuditPolicy"
)

type auditPolicyPatchHandler struct {
	decoder runtime.Decoder
}

var (
	_ handlers.Named           = &auditPolicyPatchHandler{}
	_ mutation.GeneratePatches = &auditPolicyPatchHandler{}

	//go:embed embedded/apiserver-audit-policy.yaml
	auditPolicy string
)

const auditPolicyPath = "/etc/kubernetes/audit-policy/apiserver-audit-policy.yaml"

func NewPatch() *auditPolicyPatchHandler {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &auditPolicyPatchHandler{
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
	}
}

func (h *auditPolicyPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *auditPolicyPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	topologymutation.WalkTemplates(
		ctx,
		h.decoder,
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			log := ctrl.LoggerFrom(ctx).WithValues(
				"holderRef", holderRef,
			)

			return patches.Generate(
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

					if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
						obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
					}
					apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer
					if apiServer.ExtraArgs == nil {
						apiServer.ExtraArgs = make(map[string]string, 5)
					}

					apiServer.ExtraArgs["audit-log-path"] = "/var/log/audit/kube-apiserver-audit.log"
					apiServer.ExtraArgs["audit-log-maxage"] = "30"
					apiServer.ExtraArgs["audit-log-maxbackup"] = "10"
					apiServer.ExtraArgs["audit-log-maxsize"] = "100"
					apiServer.ExtraArgs["audit-policy-file"] = auditPolicyPath

					if apiServer.ExtraVolumes == nil {
						apiServer.ExtraVolumes = make([]bootstrapv1.HostPathMount, 0, 2)
					}

					apiServer.ExtraVolumes = append(
						apiServer.ExtraVolumes,
						bootstrapv1.HostPathMount{
							Name:      "audit-policy",
							HostPath:  "/etc/kubernetes/audit-policy/",
							MountPath: "/etc/kubernetes/audit-policy/",
							ReadOnly:  true,
						},
						bootstrapv1.HostPathMount{
							Name:      "audit-logs",
							HostPath:  "/var/log/kubernetes/audit",
							MountPath: "/var/log/audit/",
						},
					)

					return nil
				},
			)
		},
	)
}
