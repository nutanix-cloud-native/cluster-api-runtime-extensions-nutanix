// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	capdv1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/serializer"
)

const (
	ClusterName                                  = "test-cluster"
	kubeadmConfigTemplateRequestObjectName       = "test-kubeadmconfigtemplate"
	kubeadmControlPlaneTemplateRequestObjectName = "test-kubeadmcontrolplanetemplate"
	Namespace                                    = corev1.NamespaceDefault
)

// NewRequestItem returns a GeneratePatchesRequestItem with the given variables and object.
func NewRequestItem(
	object runtime.Object,
	holderRef *runtimehooksv1.HolderReference,
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	if uid == "" {
		uid = uuid.NewUUID()
	}

	return runtimehooksv1.GeneratePatchesRequestItem{
		UID: uid,
		Object: runtime.RawExtension{
			Raw: serializer.ToJSON(object),
		},
		HolderReference: *holderRef,
	}
}

func NewKubeadmConfigTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return NewKubeadmConfigTemplateRequest(uid, kubeadmConfigTemplateRequestObjectName)
}

func NewKubeadmConfigTemplateRequest(
	uid types.UID,
	name string,
) runtimehooksv1.GeneratePatchesRequestItem {
	return NewRequestItem(
		&bootstrapv1.KubeadmConfigTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: bootstrapv1.GroupVersion.String(),
				Kind:       "KubeadmConfigTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: Namespace,
			},
			Spec: bootstrapv1.KubeadmConfigTemplateSpec{
				Template: bootstrapv1.KubeadmConfigTemplateResource{
					Spec: bootstrapv1.KubeadmConfigSpec{
						PostKubeadmCommands: []string{"initial-post-kubeadm"},
						JoinConfiguration: &bootstrapv1.JoinConfiguration{
							NodeRegistration: bootstrapv1.NodeRegistrationOptions{},
						},
					},
				},
			},
		},
		&runtimehooksv1.HolderReference{
			Kind:      "MachineDeployment",
			FieldPath: "spec.template.spec.infrastructureRef",
		},
		uid,
	)
}

func NewKubeadmControlPlaneTemplateRequest(
	uid types.UID,
	name string,
) runtimehooksv1.GeneratePatchesRequestItem {
	return NewRequestItem(
		&controlplanev1.KubeadmControlPlaneTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: controlplanev1.GroupVersion.String(),
				Kind:       "KubeadmControlPlaneTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: Namespace,
			},
			Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
				Template: controlplanev1.KubeadmControlPlaneTemplateResource{
					Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
						KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
							InitConfiguration: &bootstrapv1.InitConfiguration{
								NodeRegistration: bootstrapv1.NodeRegistrationOptions{},
							},
							JoinConfiguration: &bootstrapv1.JoinConfiguration{
								NodeRegistration: bootstrapv1.NodeRegistrationOptions{},
							},
						},
					},
				},
			},
		},
		&runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			FieldPath:  "spec.controlPlaneRef",
			Name:       ClusterName,
			Namespace:  Namespace,
		},
		uid,
	)
}

func NewKubeadmControlPlaneTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return NewKubeadmControlPlaneTemplateRequest(uid, kubeadmControlPlaneTemplateRequestObjectName)
}

func NewCPDockerMachineTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return NewRequestItem(
		&capdv1.DockerMachineTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: capdv1.GroupVersion.String(),
				Kind:       "DockerMachineTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-machine-template",
				Namespace: "docker-cluster",
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

func NewWorkerDockerMachineTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return NewRequestItem(
		&capdv1.DockerMachineTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: capdv1.GroupVersion.String(),
				Kind:       "DockerMachineTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-machine-template",
				Namespace: "docker-cluster",
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
