// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dns

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

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/capi/clustertopology/variables"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "DNSPatch"
)

type dnsPatchHandler struct {
	decoder runtime.Decoder
}

var (
	_ handlers.NamedHandler                   = &dnsPatchHandler{}
	_ handlers.GeneratePatchesMutationHandler = &dnsPatchHandler{}
)

func NewPatch() *dnsPatchHandler {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &dnsPatchHandler{
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
	}
}

func (h *dnsPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *dnsPatchHandler) GeneratePatches(
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

			dnsVar, found, err := variables.Get[DNSVariable](
				vars,
				VariableName,
			)
			if err != nil {
				return err
			}
			if !found {
				log.V(5).Info("DNS image repo variable not defined")
				return nil
			}

			log = log.WithValues(
				"variableName",
				VariableName,
				"variableValue",
				dnsVar,
			)

			return patches.Generate(
				obj, vars, &holderRef, selectors.ControlPlane(), log,
				func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
					log.WithValues(
						"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
						"patchedObjectName", client.ObjectKeyFromObject(obj),
					).Info("adding DNS image repo in kubeadm config spec")

					if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
						obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
					}
					obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.DNS.ImageRepository = dnsVar.ImageRepository

					return nil
				},
			)
		},
	)
}
