// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v2

// IPv4orIPv6CIDR is a string in CIDR notation or a single IP. Re-declared here
// because upstream Cilium defines it alongside many unrelated types in its
// types.go, which we do not vendor.
//
// +kubebuilder:validation:Format=cidr
type IPv4orIPv6CIDR string
