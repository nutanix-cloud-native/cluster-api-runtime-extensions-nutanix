// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request

import (
	"encoding/json"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	bootstrapv1beta1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1beta1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
)

func NewKubeadmConfigTemplateV1Beta1RequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return NewKubeadmConfigTemplateV1Beta1Request(uid, kubeadmConfigTemplateRequestObjectName)
}

func NewKubeadmConfigTemplateV1Beta1Request(
	uid types.UID,
	name string,
) runtimehooksv1.GeneratePatchesRequestItem {
	return NewRequestItem(
		&bootstrapv1beta1.KubeadmConfigTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: bootstrapv1beta1.GroupVersion.String(),
				Kind:       "KubeadmConfigTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: Namespace,
			},
			Spec: bootstrapv1beta1.KubeadmConfigTemplateSpec{
				Template: bootstrapv1beta1.KubeadmConfigTemplateResource{
					Spec: bootstrapv1beta1.KubeadmConfigSpec{
						JoinConfiguration: &bootstrapv1beta1.JoinConfiguration{
							NodeRegistration: bootstrapv1beta1.NodeRegistrationOptions{
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

type KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder struct {
	files              []bootstrapv1beta1.File
	version            *string
	apiServerExtraArgs map[string]string
}

func (b *KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder) WithFiles(
	files ...bootstrapv1beta1.File,
) *KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder {
	b.files = files
	return b
}

func (b *KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder) WithKubernetesVersion(
	version string,
) *KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder {
	b.version = ptr.To(version)
	return b
}

func (b *KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder) WithAPIServerExtraArgs(
	extraArgs map[string]string,
) *KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder {
	b.apiServerExtraArgs = extraArgs
	return b
}

func (b *KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder) NewRequest(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	cpTemplate := &controlplanev1beta1.KubeadmControlPlaneTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: controlplanev1beta1.GroupVersion.String(),
			Kind:       "KubeadmControlPlaneTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadmControlPlaneTemplateRequestObjectName,
			Namespace: Namespace,
		},
		Spec: controlplanev1beta1.KubeadmControlPlaneTemplateSpec{
			Template: controlplanev1beta1.KubeadmControlPlaneTemplateResource{
				Spec: controlplanev1beta1.KubeadmControlPlaneTemplateResourceSpec{
					KubeadmConfigSpec: bootstrapv1beta1.KubeadmConfigSpec{
						InitConfiguration: &bootstrapv1beta1.InitConfiguration{
							NodeRegistration: bootstrapv1beta1.NodeRegistrationOptions{
								KubeletExtraArgs: map[string]string{
									"cloud-provider": "external",
								},
							},
						},
						JoinConfiguration: &bootstrapv1beta1.JoinConfiguration{
							NodeRegistration: bootstrapv1beta1.NodeRegistrationOptions{
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
			cpTemplate.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1beta1.ClusterConfiguration{}
		}
		cpTemplate.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs = b.apiServerExtraArgs
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
			map[string]any{
				"controlPlane": map[string]any{
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

func NewKubeadmControlPlaneTemplateV1Beta1RequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	builder := &KubeadmControlPlaneTemplateV1Beta1RequestItemBuilder{}
	return builder.NewRequest(uid)
}

func (b *KubeadmControlPlaneTemplateRequestItemBuilder) WithAPIServerExtraV1Beta1Args(
	extraArgs map[string]string,
) *KubeadmControlPlaneTemplateRequestItemBuilder {
	args := make([]bootstrapv1.Arg, 0, len(extraArgs))
	for k, v := range extraArgs {
		args = append(args, bootstrapv1.Arg{Name: k, Value: ptr.To(v)})
	}
	return b.WithAPIServerExtraArgs(args)
}
