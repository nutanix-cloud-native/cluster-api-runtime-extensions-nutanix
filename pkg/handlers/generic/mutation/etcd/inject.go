// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	"crypto/tls"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "etcd"
)

type etcdPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *etcdPatchHandler {
	return newEtcdPatchHandler(v1alpha1.ClusterConfigVariableName, VariableName)
}

func newEtcdPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *etcdPatchHandler {
	return &etcdPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

var defaultEtcdExtraArgs = map[string]string{
	"auto-tls":      "false",
	"peer-auto-tls": "false",
	"cipher-suites": strings.Join(
		[]string{
			tls.CipherSuiteName(tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256),
			tls.CipherSuiteName(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256),
			tls.CipherSuiteName(tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384),
			tls.CipherSuiteName(tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384),
		},
		",",
	),
	"tls-min-version": "TLS1.2",
}

func (h *etcdPatchHandler) Mutate(
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

	etcd, err := variables.Get[v1alpha1.Etcd](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil && !variables.IsNotFoundError(err) {
		return err
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		etcd,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
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

			if localEtcd.ExtraArgs == nil {
				localEtcd.ExtraArgs = make(map[string]string, len(defaultEtcdExtraArgs))
			}

			for k, v := range defaultEtcdExtraArgs {
				if _, ok := localEtcd.ExtraArgs[k]; !ok {
					localEtcd.ExtraArgs[k] = v
				}
			}

			if etcd.Image == nil {
				return nil
			}

			if etcd.Image.Tag != "" {
				localEtcd.ImageTag = etcd.Image.Tag
			}
			if etcd.Image.Repository != "" {
				localEtcd.ImageRepository = etcd.Image.Repository
			}

			return nil
		},
	)
}
