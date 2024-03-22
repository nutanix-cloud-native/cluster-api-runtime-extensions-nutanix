// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "extraAPIServerCertSANs"
)

type extraAPIServerCertSANsPatchHandler struct {
	variableName      string
	variableFieldPath []string
	client            ctrlclient.Reader
}

func NewPatch(
	cl ctrlclient.Reader,
) *extraAPIServerCertSANsPatchHandler {
	scheme := runtime.NewScheme()
	_ = capiv1.AddToScheme(scheme)
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return newExtraAPIServerCertSANsPatchHandler(clusterconfig.MetaVariableName, cl, VariableName)
}

func newExtraAPIServerCertSANsPatchHandler(
	variableName string,
	cl ctrlclient.Reader,
	variableFieldPath ...string,
) *extraAPIServerCertSANsPatchHandler {
	return &extraAPIServerCertSANsPatchHandler{
		variableName:      variableName,
		client:            cl,
		variableFieldPath: variableFieldPath,
	}
}

func (h *extraAPIServerCertSANsPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey ctrlclient.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)
	cluster := &capiv1.Cluster{}
	if err := h.client.Get(ctx, clusterKey, cluster); err != nil {
		return err
	}
	defaultAPICertSANs := getDefaultAPIServerSANs(cluster)
	extraAPIServerCertSANsVar, found, err := variables.Get[v1alpha1.ExtraAPIServerCertSANs](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found && len(defaultAPICertSANs) == 0 {
		log.V(5).Info("No Extra API server cert SANs needed to be added")
		return nil
	}

	extraSans := append(extraAPIServerCertSANsVar, defaultAPICertSANs...)

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		extraAPIServerCertSANsVar,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding API server extra cert SANs in kubeadm config spec")

			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
			}
			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.CertSANs = extraSans
			return nil
		},
	)
}

func getDefaultAPIServerSANs(cluster *capiv1.Cluster) []string {
	provider, ok := cluster.Labels[capiv1.ProviderNameLabel]
	if !ok {
		return []string{}
	}
	switch provider {
	case "docker":
		return v1alpha1.DefaultDockerCertSANs
	default:
		return []string{}
	}
}
