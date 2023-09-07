// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package selectors

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
)

func ControlPlane() clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: controlplanev1.GroupVersion.String(),
		Kind:       "KubeadmControlPlaneTemplate",
		MatchResources: clusterv1.PatchSelectorMatch{
			ControlPlane: true,
		},
	}
}

func DefaultWorkerSelector() clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: bootstrapv1.GroupVersion.String(),
		Kind:       "KubeadmConfigTemplate",
		MatchResources: clusterv1.PatchSelectorMatch{
			MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
				Names: []string{
					"default-worker",
				},
			},
		},
	}
}

func AllWorkersSelector() clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: bootstrapv1.GroupVersion.String(),
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
