// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"fmt"
	"net/netip"
	"strings"

	"go4.org/netipx"
)

// IsIPInRange returns true if target IP falls within the IP range(inclusive of start and end IP),
// CIDR(inclusive of start and end IP), single IP.
func IsIPInRange(ipAddr, targetIP string) (bool, error) {
	parsedTargetIP, err := netip.ParseAddr(targetIP)
	if err != nil {
		return false, fmt.Errorf("failed to parse target IP %q: %v", targetIP, err)
	}

	switch {
	case strings.Contains(ipAddr, "-"):
		ipRange, err := netipx.ParseIPRange(ipAddr)
		if err != nil {
			return false, fmt.Errorf("failed to parse IP range %q: %v", ipAddr, err)
		}
		return ipRange.Contains(parsedTargetIP), nil

	case strings.Contains(ipAddr, "/"):
		prefix, err := netip.ParsePrefix(ipAddr)
		if err != nil {
			return false, fmt.Errorf("failed to parse IP prefix %q: %v", ipAddr, err)
		}
		return prefix.Contains(parsedTargetIP), nil

	default:
		parsedIP, err := netip.ParseAddr(ipAddr)
		if err != nil {
			return false, fmt.Errorf("failed to parse IP address %q: %v", ipAddr, err)
		}
		return parsedIP.Compare(parsedTargetIP) == 0, nil
	}
}
