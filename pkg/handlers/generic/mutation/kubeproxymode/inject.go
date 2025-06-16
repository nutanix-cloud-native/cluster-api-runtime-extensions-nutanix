// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeproxymode

import (
	"context"
	"fmt"

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
	VariableName = "kubeProxy"
)

type kubeProxyMode struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *kubeProxyMode {
	return newKubeProxyModePatch(
		v1alpha1.ClusterConfigVariableName,
		VariableName,
		"mode",
	)
}

func newKubeProxyModePatch(
	variableName string,
	variableFieldPath ...string,
) *kubeProxyMode {
	return &kubeProxyMode{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *kubeProxyMode) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	clusterGetter mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	kubeProxyMode, err := variables.Get[v1alpha1.KubeProxyMode](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("kubeProxy mode variable not defined")
			return nil
		}

		return err
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		kubeProxyMode,
	)

	if kubeProxyMode == "" {
		log.V(5).Info("kube proxy mode is not set, skipping mutation")
		return nil
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding kube proxy mode to control plane kubeadm config spec")
			if obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration = &bootstrapv1.InitConfiguration{}
			}

			switch kubeProxyMode {
			case v1alpha1.KubeProxyModeDisabled:
				log.Info("kube proxy mode is set to disabled, skipping kube-proxy addon")
				obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration.SkipPhases = append(
					obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration.SkipPhases,
					"addon/kube-proxy",
				)
			case v1alpha1.KubeProxyModeIPTables:
				log.Info(
					"kube proxy mode is set to iptables, no patches required as this is the default mode configured by kubeadm",
				)
			default:
				return fmt.Errorf("unknown kube proxy mode %q", kubeProxyMode)
			}

			return nil
		},
	)
}
