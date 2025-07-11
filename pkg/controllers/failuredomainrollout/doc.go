// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package failuredomainrollout provides a controller that monitors cluster.status.failureDomains
// and triggers rollouts on KubeAdmControlPlane when there are meaningful changes to failure domains
// that impact the current control plane placement.
//
// A meaningful change is defined as:
// - A failure domain currently in use by the control plane being disabled (controlPlane: false)
// - A failure domain currently in use by the control plane being removed
// - A failure domain is overutilized and can be improved by spreading the control plane across more failure domains
//
// The controller does NOT trigger rollouts when:
// - New failure domains are added but existing ones remain valid
// - Changes to failure domains that are not currently in use by the control plane
//
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch
package failuredomainrollout
