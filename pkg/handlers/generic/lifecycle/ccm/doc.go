// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package calico provides a handler for managing Calico deployments on clusters, configurable via
// labels and annotations.
//
// To enable Calico deployment, a cluster must be labelled with `capiext.labs.d2iq.io/cni=calico`.
// This will ensure the Tigera Configmap and associated ClusterResourceSet.
//
// +kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=clusterresourcesets,verbs=watch;list;get;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=watch;list;get;create;patch;update;delete
package ccm
