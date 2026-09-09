// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +groupName=cilium.io

// Package v2alpha1 is a minimal, in-tree vendoring of Cilium's
// cilium.io/v2alpha1 API. Only CiliumL2AnnouncementPolicy (and its List) are
// exposed here.
//
// The l2announcement_types.go file is synced verbatim from upstream Cilium
// via `make api.sync.cilium`; the slim/... LabelSelector reference is
// rewritten to apimachinery's meta/v1.LabelSelector, which has the same wire
// shape. register.go and the generated deepcopy are authored here.
package v2alpha1
