// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +groupName=cilium.io

// Package v2 is a minimal, in-tree vendoring of Cilium's cilium.io/v2 API.
// Only CiliumLoadBalancerIPPool (and its List) are exposed here because
// upstream's kitchen-sink package drags in ~30 internal Cilium packages that
// would conflict with sigs.k8s.io/cluster-api's dependencies.
//
// The lbipam_types.go file is synced verbatim from upstream Cilium via
// `make api.sync.cilium`; the slim/... LabelSelector reference is rewritten
// to apimachinery's meta/v1.LabelSelector, which has the same wire shape.
// Everything else in this package (this file, register.go, types.go, and
// the generated deepcopy) is authored here.
package v2
