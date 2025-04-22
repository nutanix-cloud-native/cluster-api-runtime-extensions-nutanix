// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=watch;list;get;update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=watch;list;get;create;patch;update;delete
package inclusterregistry
