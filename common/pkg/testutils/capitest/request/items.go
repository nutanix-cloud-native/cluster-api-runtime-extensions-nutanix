// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request

import (
	"encoding/json"
	"maps"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
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
							NodeRegistration: bootstrapv1.NodeRegistrationOptions{
								KubeletExtraArgs: map[string]string{
									"cloud-provider": "external",
								},
							},
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

type KubeadmControlPlaneTemplateRequestItemBuilder struct {
	files              []bootstrapv1.File
	version            *string
	apiServerExtraArgs map[string]string
}

func (b *KubeadmControlPlaneTemplateRequestItemBuilder) WithFiles(
	files ...bootstrapv1.File,
) *KubeadmControlPlaneTemplateRequestItemBuilder {
	b.files = files
	return b
}

func (b *KubeadmControlPlaneTemplateRequestItemBuilder) WithKubernetesVersion(
	version string,
) *KubeadmControlPlaneTemplateRequestItemBuilder {
	b.version = ptr.To(version)
	return b
}

func (b *KubeadmControlPlaneTemplateRequestItemBuilder) WithAPIServerExtraArgs(
	extraArgs map[string]string,
) *KubeadmControlPlaneTemplateRequestItemBuilder {
	b.apiServerExtraArgs = extraArgs
	return b
}

func (b *KubeadmControlPlaneTemplateRequestItemBuilder) NewRequest(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	cpTemplate := &controlplanev1.KubeadmControlPlaneTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KubeadmControlPlaneTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadmControlPlaneTemplateRequestObjectName,
			Namespace: Namespace,
		},
		Spec: controlplanev1.KubeadmControlPlaneTemplateSpec{
			Template: controlplanev1.KubeadmControlPlaneTemplateResource{
				Spec: controlplanev1.KubeadmControlPlaneTemplateResourceSpec{
					KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
						InitConfiguration: &bootstrapv1.InitConfiguration{
							NodeRegistration: bootstrapv1.NodeRegistrationOptions{
								KubeletExtraArgs: map[string]string{
									"cloud-provider": "external",
								},
							},
						},
						JoinConfiguration: &bootstrapv1.JoinConfiguration{
							NodeRegistration: bootstrapv1.NodeRegistrationOptions{
								KubeletExtraArgs: map[string]string{
									"cloud-provider": "external",
								},
							},
						},
						Files: b.files,
					},
				},
			},
		},
	}

	if b.apiServerExtraArgs != nil {
		if cpTemplate.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
			cpTemplate.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
		}
		clusterConfiguration := cpTemplate.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration
		clusterConfiguration.APIServer.ExtraArgs = maps.Clone(b.apiServerExtraArgs)
	}

	requestItem := NewRequestItem(
		cpTemplate,
		&runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			FieldPath:  "spec.controlPlaneRef",
			Name:       ClusterName,
			Namespace:  Namespace,
		},
		uid,
	)

	if b.version != nil {
		marshaledBuiltin, _ := json.Marshal( //nolint:errchkjson // Marshalling is guaranteed to succeed.
			map[string]interface{}{
				"controlPlane": map[string]interface{}{
					"version": *b.version,
				},
			},
		)
		requestItem.Variables = append(requestItem.Variables, runtimehooksv1.Variable{
			Name:  runtimehooksv1.BuiltinsName,
			Value: apiextensionsv1.JSON{Raw: marshaledBuiltin},
		})
	}

	return requestItem
}

func NewKubeadmControlPlaneTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	builder := &KubeadmControlPlaneTemplateRequestItemBuilder{}
	return builder.NewRequest(uid)
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
