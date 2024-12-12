// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"fmt"
	"net/netip"
)

// IsIPInRange checks if the target IP falls within the start and end IP range (inclusive).
func IsIPInRange(startIP, endIP, targetIP string) (bool, error) {
	start, err := netip.ParseAddr(startIP)
	if err != nil {
		return false, fmt.Errorf("invalid start IP: %w", err)
	}
	end, err := netip.ParseAddr(endIP)
	if err != nil {
		return false, fmt.Errorf("invalid end IP: %w", err)
	}
	target, err := netip.ParseAddr(targetIP)
	if err != nil {
		return false, fmt.Errorf("invalid target IP: %w", err)
	}

	return start.Compare(target) <= 0 && end.Compare(target) >= 0, nil
}
