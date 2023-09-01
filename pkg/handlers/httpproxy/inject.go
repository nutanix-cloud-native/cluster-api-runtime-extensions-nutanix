// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/capi/clustertopology/variables"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "HTTPProxyPatch"
)

type httpProxyPatchHandler struct {
	decoder runtime.Decoder
}

var (
	_ handlers.Named           = &httpProxyPatchHandler{}
	_ mutation.GeneratePatches = &httpProxyPatchHandler{}
)

func NewPatch() *httpProxyPatchHandler {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &httpProxyPatchHandler{
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
	}
}

func (h *httpProxyPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *httpProxyPatchHandler) GeneratePatches(
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

			httpProxyVariable, found, err := variables.Get[HTTPProxyVariables](
				vars,
				VariableName,
			)
			if err != nil {
				return err
			}
			if !found {
				log.Info("http proxy variable not defined")
				return nil
			}

			log = log.WithValues("variableName", VariableName, "variableValue", httpProxyVariable)

			if err := patches.Generate(
				obj, vars, &holderRef, selectors.ControlPlane(), log,
				func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
					log.WithValues(
						"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
						"patchedObjectName", client.ObjectKeyFromObject(obj),
					).Info("adding files to control plane kubeadm config spec")
					obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
						obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
						generateSystemdFiles(httpProxyVariable)...,
					)
					return nil
				}); err != nil {
				return err
			}

			if err := patches.Generate(
				obj, vars, &holderRef, selectors.AllWorkersSelector(), log,
				func(obj *bootstrapv1.KubeadmConfigTemplate) error {
					log.WithValues(
						"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
						"patchedObjectName", client.ObjectKeyFromObject(obj),
					).Info("adding files to worker node kubeadm config template")
					obj.Spec.Template.Spec.Files = append(
						obj.Spec.Template.Spec.Files,
						generateSystemdFiles(httpProxyVariable)...,
					)
					return nil
				}); err != nil {
				return err
			}

			return nil
		},
	)
}
