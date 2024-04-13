// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerapiservercertsans

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
)

var DefaultDockerCertSANs = []string{
	"localhost",
	"127.0.0.1",
	"0.0.0.0",
	"host.docker.internal",
}

type dockerAPIServerCertSANSPatchHandler struct{}

func NewPatch() *dockerAPIServerCertSANSPatchHandler {
	return &dockerAPIServerCertSANSPatchHandler{}
}

func (h *dockerAPIServerCertSANSPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey ctrlclient.ObjectKey,
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
			).Info("adding docker default API server extra cert SANs in kubeadm config spec")

			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
			}
			certSANs := obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.CertSANs
			if certSANs == nil {
				certSANs = []string{}
			}
			certSANs = append(certSANs, DefaultDockerCertSANs...)
			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.CertSANs = certSANs
			return nil
		},
	)
}
