// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package selectors

import (
	"k8s.io/utils/ptr"
	bootstrapv1beta1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	controlplanev1beta1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func V1Beta1ControlPlane() clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: controlplanev1beta1.GroupVersion.String(),
		Kind:       "KubeadmControlPlaneTemplate",
		MatchResources: clusterv1.PatchSelectorMatch{
			ControlPlane: ptr.To(true),
		},
	}
}

func V1Beta1WorkersKubeadmConfigTemplateSelector() clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: bootstrapv1beta1.GroupVersion.String(),
		Kind:       "KubeadmConfigTemplate",
		MatchResources: clusterv1.PatchSelectorMatch{
			MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
				Names: []string{
					"*",
				},
			},
		},
	}
}
