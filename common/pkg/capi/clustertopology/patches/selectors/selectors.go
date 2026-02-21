// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package selectors

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func ControlPlane() clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: controlplanev1.GroupVersion.String(),
		Kind:       "KubeadmControlPlaneTemplate",
		MatchResources: clusterv1.PatchSelectorMatch{
			ControlPlane: ptr.To(true),
		},
	}
}

func WorkersKubeadmConfigTemplateSelector() clusterv1.PatchSelector {
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

func WorkersConfigTemplateSelector(capiInfrastructureAPIVersion, kind string) clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: capiInfrastructureAPIVersion,
		Kind:       kind,
		MatchResources: clusterv1.PatchSelectorMatch{
			MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
				Names: []string{
					"*",
				},
			},
		},
	}
}

// InfrastructureCluster selector matches against infrastructure clusters.
// Passing in the API version (not the API group) is required because different providers could support different API
// versions. This also allows for a patch to select multiple infrastructure versions for the same provider.
func InfrastructureCluster(capiInfrastructureAPIVersion, kind string) clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: schema.GroupVersion{
			Group:   "infrastructure.cluster.x-k8s.io",
			Version: capiInfrastructureAPIVersion,
		}.String(),
		Kind: kind,
		MatchResources: clusterv1.PatchSelectorMatch{
			InfrastructureCluster: ptr.To(true),
		},
	}
}

// InfrastructureWorkerMachineTemplates selector matches against infrastructure machines.
// Passing in the API version (not the API group) is required because different providers could support different API
// versions. This also allows for a patch to select multiple infrastructure versions for the same provider.
func InfrastructureWorkerMachineTemplates(
	capiInfrastructureAPIVersion, kind string,
) clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: schema.GroupVersion{
			Group:   "infrastructure.cluster.x-k8s.io",
			Version: capiInfrastructureAPIVersion,
		}.String(),
		Kind: kind,
		MatchResources: clusterv1.PatchSelectorMatch{
			MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
				Names: []string{"*"},
			},
		},
	}
}

// InfrastructureControlPlaneMachines selector matches against infrastructure control-plane machines.
// Passing in the API version (not the API group) is required because different providers could support different API
// versions. This also allows for a patch to select multiple infrastructure versions for the same provider.
func InfrastructureControlPlaneMachines(
	capiInfrastructureAPIVersion, kind string,
) clusterv1.PatchSelector {
	return clusterv1.PatchSelector{
		APIVersion: schema.GroupVersion{
			Group:   "infrastructure.cluster.x-k8s.io",
			Version: capiInfrastructureAPIVersion,
		}.String(),
		Kind: kind,
		MatchResources: clusterv1.PatchSelectorMatch{
			ControlPlane: ptr.To(true),
		},
	}
}
