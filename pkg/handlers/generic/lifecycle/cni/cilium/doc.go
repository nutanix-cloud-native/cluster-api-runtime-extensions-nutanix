// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cilium provides a handler for managing Cilium deployments on clusters, configurable via
// labels and annotations.
//
// To enable Calico deployment, a cluster must be labelled with `capiext.labs.d2iq.io/cni=cilium`.
// This will ensure the Tigera Configmap and associated ClusterResourceSet.
//
// +kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=watch;list;get;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=watch;list;get
package cilium
