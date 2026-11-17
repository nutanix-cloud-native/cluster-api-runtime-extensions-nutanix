// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// The endpoint mutations read the address the IPAM provider allocated, because
// it is never written back into the Cluster.
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddresses,verbs=get;list;watch
package ipamendpoint
