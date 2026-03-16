// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func NewNutanixClusterTemplateRequestItem(
	uid types.UID,
	existingSpec ...capxv1.NutanixClusterTemplateSpec,
) runtimehooksv1.GeneratePatchesRequestItem {
	nutanixClusterTemplate := &capxv1.NutanixClusterTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: capxv1.GroupVersion.String(),
			Kind:       "NutanixClusterTemplate",
		},
	}

	switch len(existingSpec) {
	case 0:
		// Do nothing.
	case 1:
		nutanixClusterTemplate.Spec = existingSpec[0]
	default:
		panic("can only take at most one existing spec")
	}

	return request.NewRequestItem(
		nutanixClusterTemplate,
		&runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			FieldPath:  "spec.infrastructureRef",
			Name:       request.ClusterName,
			Namespace:  request.Namespace,
		},
		uid,
	)
}

func NewCPNutanixMachineTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return request.NewRequestItem(
		&capxv1.NutanixMachineTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: capxv1.GroupVersion.String(),
				Kind:       "NutanixMachineTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nutanix-machine-template",
				Namespace: "nutanix-cluster",
			},
		},
		&runtimehooksv1.HolderReference{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KubeadmControlPlane",
			FieldPath:  "spec.machineTemplate.infrastructureRef",
		},
		uid,
	)
}

func NewWorkerNutanixMachineTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return request.NewRequestItem(
		&capxv1.NutanixMachineTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: capxv1.GroupVersion.String(),
				Kind:       "NutanixMachineTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nutanix-machine-template",
				Namespace: "nutanix-cluster",
			},
		},
		&runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			FieldPath:  "spec.template.spec.infrastructureRef",
		},
		uid,
	)
}
