// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

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

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "ExtraAPIServerCertSANsPatch"
)

type extraAPIServerCertSANsPatchHandler struct {
	decoder           runtime.Decoder
	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &extraAPIServerCertSANsPatchHandler{}
	_ mutation.GeneratePatches = &extraAPIServerCertSANsPatchHandler{}
)

func NewPatch() *extraAPIServerCertSANsPatchHandler {
	return newExtraAPIServerCertSANsPatchHandler(variableName)
}

func NewMetaPatch() *extraAPIServerCertSANsPatchHandler {
	return newExtraAPIServerCertSANsPatchHandler(handlers.MetaVariableName, variableName)
}

func newExtraAPIServerCertSANsPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *extraAPIServerCertSANsPatchHandler {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &extraAPIServerCertSANsPatchHandler{
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *extraAPIServerCertSANsPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *extraAPIServerCertSANsPatchHandler) GeneratePatches(
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

			extraAPIServerCertSANsVar, found, err := variables.Get[v1alpha1.ExtraAPIServerCertSANs](
				vars,
				h.variableName,
				h.variableFieldPath...,
			)
			if err != nil {
				return err
			}
			if !found {
				log.V(5).Info("Extra API server cert SANs variable not defined")
				return nil
			}

			log = log.WithValues(
				"variableName",
				h.variableName,
				"variableFieldPath",
				h.variableFieldPath,
				"variableValue",
				extraAPIServerCertSANsVar,
			)

			return patches.Generate(
				obj, vars, &holderRef, selectors.ControlPlane(), log,
				func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
					log.WithValues(
						"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
						"patchedObjectName", client.ObjectKeyFromObject(obj),
					).Info("adding API server extra cert SANs in kubeadm config spec")

					if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
						obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
					}
					obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.CertSANs = extraAPIServerCertSANsVar

					return nil
				},
			)
		},
	)
}
