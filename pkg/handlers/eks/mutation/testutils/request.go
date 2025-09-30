// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	eksbootstrapv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/bootstrap/eks/api/v1beta2"
	eksv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func NewEKSControlPlaneRequestItem(
	uid types.UID,
	existingSpec ...eksv1.AWSManagedControlPlaneTemplateSpec,
) runtimehooksv1.GeneratePatchesRequestItem {
	eksClusterTemplate := &eksv1.AWSManagedControlPlaneTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eksv1.GroupVersion.String(),
			Kind:       "AWSManagedControlPlaneTemplate",
		},
	}

	switch len(existingSpec) {
	case 0:
		// Do nothing.
	case 1:
		eksClusterTemplate.Spec = existingSpec[0]
	default:
		panic("can only take at most one existing spec")
	}

	return request.NewRequestItem(
		eksClusterTemplate,
		&runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			FieldPath:  "spec.controlPlaneRef",
			Name:       request.ClusterName,
			Namespace:  request.Namespace,
		},
		uid,
	)
}

func NewNodeadmConfigTemplateRequestItem(
	uid types.UID,
	existingSpec ...eksbootstrapv1.NodeadmConfigTemplateSpec,
) runtimehooksv1.GeneratePatchesRequestItem {
	nodeadmConfigTemplate := &eksbootstrapv1.NodeadmConfigTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eksbootstrapv1.GroupVersion.String(),
			Kind:       "NodeadmConfigTemplate",
		},
	}

	switch len(existingSpec) {
	case 0:
		// Do nothing.
	case 1:
		nodeadmConfigTemplate.Spec = existingSpec[0]
	default:
		panic("can only take at most one existing spec")
	}

	return request.NewRequestItem(
		nodeadmConfigTemplate,
		&runtimehooksv1.HolderReference{
			Kind:      "MachineDeployment",
			FieldPath: "spec.template.spec.bootstrap.configRef",
		},
		uid,
	)
}
