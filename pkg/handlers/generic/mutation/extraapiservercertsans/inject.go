// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

import (
	"context"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "extraAPIServerCertSANs"
)

type extraAPIServerCertSANsPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *extraAPIServerCertSANsPatchHandler {
	return newExtraAPIServerCertSANsPatchHandler(clusterconfig.MetaVariableName, VariableName)
}

func newExtraAPIServerCertSANsPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *extraAPIServerCertSANsPatchHandler {
	return &extraAPIServerCertSANsPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *extraAPIServerCertSANsPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey client.ObjectKey,
	clusterGetter mutation.ClusterGetter,
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
		log.Error(
			err,
			"failed to get cluster config variable from extraAPIServerCertSANs mutation handler",
		)
	}
	if !found {
		log.V(5).Info("Extra API server cert SANs variable not defined")
	}
	cluster, err := clusterGetter(ctx)
	if err != nil {
		log.Error(
			err,
			"failed to get cluster from extraAPIServerCertSANs mutation handler",
		)
	}
	defaultAPICertSANs := getDefaultAPIServerSANs(cluster)
	apiCertSANs := slices.Clone[[]string](extraAPIServerCertSANsVar)
	apiCertSANs = append(apiCertSANs, defaultAPICertSANs...)
	apiCertSANs = slices.Compact(apiCertSANs)
	if len(apiCertSANs) == 0 {
		log.Info("No APIServerSANs to apply")
		return nil
	}
	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		apiCertSANs,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding API server extra cert SANs in kubeadm config spec")

			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
			}
			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.CertSANs = apiCertSANs
			return nil
		},
	)
}

func getDefaultAPIServerSANs(cluster *clusterv1.Cluster) []string {
	switch utils.GetProvider(cluster) {
	case "docker":
		return v1alpha1.DefaultDockerCertSANs
	default:
		return nil
	}
}
