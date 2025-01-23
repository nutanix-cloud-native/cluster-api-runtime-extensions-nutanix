// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package request

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func NewAWSClusterTemplateRequestItem(
	uid types.UID,
	existingSpec ...capav1.AWSClusterTemplateSpec,
) runtimehooksv1.GeneratePatchesRequestItem {
	awsClusterTemplate := &capav1.AWSClusterTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: capav1.GroupVersion.String(),
			Kind:       "AWSClusterTemplate",
		},
	}

	switch len(existingSpec) {
	case 0:
		// Do nothing.
	case 1:
		awsClusterTemplate.Spec = existingSpec[0]
	default:
		panic("can only take at most one existing spec")
	}

	return request.NewRequestItem(
		awsClusterTemplate,
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

func NewCPAWSMachineTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return request.NewRequestItem(
		&capav1.AWSMachineTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: capav1.GroupVersion.String(),
				Kind:       "AWSMachineTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "aws-machine-template",
				Namespace: "aws-cluster",
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

func NewWorkerAWSMachineTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return request.NewRequestItem(
		&capav1.AWSMachineTemplate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: capav1.GroupVersion.String(),
				Kind:       "AWSMachineTemplate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "aws-machine-template",
				Namespace: "aws-cluster",
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
