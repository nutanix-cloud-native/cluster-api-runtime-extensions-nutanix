// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// The handlers set the Cluster as owner of credential Secrets. With the
// OwnerReferencesPermissionEnforcement admission plugin enabled, setting an
// ownerReference requires delete permission on the owned object.
// +kubebuilder:rbac:groups="",resources=secrets,verbs=watch;list;get;patch;create;update;delete
package credentials
