// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package enforceclusterautoscalerlimits provides a controller that enforces Cluster Autoscaler
// limits on MachineDeployments.
//
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch;update;patch
package enforceclusterautoscalerlimits
