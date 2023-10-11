// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package etcd

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
	HandlerNamePatch = "EtcdPatch"
)

type etcdPatchHandler struct {
	*handlers.GenericPatchHandler[*controlplanev1.KubeadmControlPlaneTemplate]
}

func NewPatch() *etcdPatchHandler {
	return newEtcdPatchHandler(VariableName)
}

func NewMetaPatch() *etcdPatchHandler {
	return newEtcdPatchHandler(clusterconfig.MetaVariableName, VariableName)
}

func newEtcdPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *etcdPatchHandler {
	return &etcdPatchHandler{
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
	return variables.Get[v1alpha1.Etcd](vars, name, fields...)
}

func mutateFunc(
	log logr.Logger,
	_ map[string]apiextensionsv1.JSON,
	patchVar any,
) func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
	return func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
		etcd := patchVar.(v1alpha1.Etcd)

		log.WithValues(
			"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
			"patchedObjectName", client.ObjectKeyFromObject(obj),
		).Info("setting etcd configuration in kubeadm config spec")

		if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
		}
		if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local == nil {
			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local = &bootstrapv1.LocalEtcd{}
		}

		localEtcd := obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.Etcd.Local
		if etcd.Image != nil && etcd.Image.Tag != "" {
			localEtcd.ImageTag = etcd.Image.Tag
		}
		if etcd.Image != nil && etcd.Image.Repository != "" {
			localEtcd.ImageRepository = etcd.Image.Repository
		}

		return nil
	}
}
