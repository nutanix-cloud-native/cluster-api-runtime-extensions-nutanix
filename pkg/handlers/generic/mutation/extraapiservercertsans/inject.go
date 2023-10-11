// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

import (
	_ "embed"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "ExtraAPIServerCertSANsPatch"
)

type extraAPIServerCertSANsPatchHandler struct {
	*handlers.GenericPatchHandler[*controlplanev1.KubeadmControlPlaneTemplate]
}

func NewPatch() *extraAPIServerCertSANsPatchHandler {
	return newExtraAPIServerCertSANsPatchHandler(VariableName)
}

func NewMetaPatch() *extraAPIServerCertSANsPatchHandler {
	return newExtraAPIServerCertSANsPatchHandler(clusterconfig.MetaVariableName, VariableName)
}

func newExtraAPIServerCertSANsPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *extraAPIServerCertSANsPatchHandler {
	return &extraAPIServerCertSANsPatchHandler{
		handlers.NewGenericPatchHandler[*controlplanev1.KubeadmControlPlaneTemplate](
			HandlerNamePatch,
			variableFunc,
			selectors.ControlPlane(),
			mutateFunc,
			variableName,
			variableFieldPath...,
		),
	}
}

func variableFunc(vars map[string]apiextensionsv1.JSON, name string, fields ...string) (any, bool, error) {
	return variables.Get[v1alpha1.ExtraAPIServerCertSANs](vars, name, fields...)
}

func mutateFunc(
	log logr.Logger,
	_ map[string]apiextensionsv1.JSON,
	patchVar any,
) func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
	return func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
		extraAPIServerCertSANsVar := patchVar.(v1alpha1.ExtraAPIServerCertSANs)

		log.WithValues(
			"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
			"patchedObjectName", client.ObjectKeyFromObject(obj),
		).Info("adding API server extra cert SANs in kubeadm config spec")

		if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
		}
		obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.CertSANs = extraAPIServerCertSANsVar

		return nil
	}
}
