// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package syncclusterclass provides a controller that copies ClusterClasses and their referenced
// Templates from a source namespace to target namespaces.
//
// For every ClusterClass in the source namespace, the controller creates a copy of it, and all its
// referenced Templates, in the target namespace.
//
// Resources in target namespaces are not updated, even if they are updated or deleted in the source
// namespace.
//
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusterclasses,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
package namespacesync
