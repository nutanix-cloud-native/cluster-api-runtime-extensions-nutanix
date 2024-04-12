// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

import (
	"context"
	"fmt"

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
}

func NewPatch() *extraAPIServerCertSANsPatchHandler {
	scheme := runtime.NewScheme()
	_ = capiv1.AddToScheme(scheme)
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
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
	clusterKey ctrlclient.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)
	clusterConfig, found, err := variables.Get[v1alpha1.ClusterConfigSpec](
		vars,
		h.variableName,
	)
	if err != nil {
		log.Error(
			err,
			"failed to get cluster config variable from extraAPIServerCertSANs mutation handler",
		)
	}
	// this really shouldn't happen, but we'll account for this case anyways
	if !found {
		log.Info("clusterConfig.Spec not found form extraAPIServerCertSANs mutation handler")
		return fmt.Errorf(
			"clusterConfig.Spec not found form extraAPIServerCertSANs mutation handler from vars %v",
			vars,
		)
	}
	defaultAPICertSANs := getDefaultAPIServerSANs(&clusterConfig)
	//nolint: gocritic // this is more readable
	apiCertSANs := append(clusterConfig.ExtraAPIServerCertSANs, defaultAPICertSANs...)
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
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding API server extra cert SANs in kubeadm config spec")

			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
			}
			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.CertSANs = apiCertSANs
			return nil
		},
	)
}

func getDefaultAPIServerSANs(spec *v1alpha1.ClusterConfigSpec) []string {
	if spec.Docker != nil && spec.AWS == nil && spec.Nutanix == nil {
		return v1alpha1.DefaultDockerCertSANs
	}
	return []string{}
}
