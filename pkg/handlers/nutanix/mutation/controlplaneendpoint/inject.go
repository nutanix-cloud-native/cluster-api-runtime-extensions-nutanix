// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneendpoint

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "controlPlaneEndpoint"
)

type nutanixControlPlaneEndpoint struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *nutanixControlPlaneEndpoint {
	return newNutanixControlPlaneEndpoint(
		clusterconfig.MetaVariableName,
		v1alpha1.NutanixVariableName,
		VariableName,
	)
}

func newNutanixControlPlaneEndpoint(
	variableName string,
	variableFieldPath ...string,
) *nutanixControlPlaneEndpoint {
	return &nutanixControlPlaneEndpoint{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *nutanixControlPlaneEndpoint) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	controlPlaneEndpointVar, found, err := variables.Get[v1alpha1.ControlPlaneEndpointSpec](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("Nutanix ControlPlaneEndpoint variable not defined")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		controlPlaneEndpointVar,
	)

	if err := patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			commands := []string{
				fmt.Sprintf("sed -i 's/control_plane_endpoint_ip/%s/g' /etc/kubernetes/manifests/kube-vip.yaml",
					controlPlaneEndpointVar.Host),
				fmt.Sprintf("sed -i 's/control_plane_endpoint_port/%d/g' /etc/kubernetes/manifests/kube-vip.yaml",
					controlPlaneEndpointVar.Port),
			}
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding PreKubeadmCommands to control plane kubeadm config spec")
			obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands,
				commands...,
			)
			return nil
		},
	); err != nil {
		return err
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster(capxv1.GroupVersion.Version, "NutanixClusterTemplate"),
		log,
		func(obj *capxv1.NutanixClusterTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting controlPlaneEndpoint in NutanixCluster spec")

			obj.Spec.Template.Spec.ControlPlaneEndpoint = capiv1.APIEndpoint{
				Host: controlPlaneEndpointVar.Host,
				Port: controlPlaneEndpointVar.Port,
			}

			return nil
		},
	)
}
