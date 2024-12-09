// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"fmt"
	"math/big"
	"net"
)

// IsIPInRange checks if the target IP falls within the start and end IP range (inclusive).
func IsIPInRange(startIP, endIP, targetIP string) (bool, error) {
	// Parse the IPs
	start := net.ParseIP(startIP)
	end := net.ParseIP(endIP)
	target := net.ParseIP(targetIP)

	// Ensure all IPs are valid
	if start == nil {
		return false, fmt.Errorf("invalid start IP: %q", startIP)
	}
	if end == nil {
		return false, fmt.Errorf("invalid end IP: %q", endIP)
	}
	if target == nil {
		return false, fmt.Errorf("invalid target IP: %q", targetIP)
	}

	// Convert IPs to big integers
	startInt := ipToBigInt(start)
	endInt := ipToBigInt(end)
	targetInt := ipToBigInt(target)

	// Check if target IP is within the range
	return targetInt.Cmp(startInt) >= 0 && targetInt.Cmp(endInt) <= 0, nil
}

// ipToBigInt converts a net.IP to a big.Int for comparison.
func ipToBigInt(ip net.IP) *big.Int {
	// Normalize to 16-byte representation for both IPv4 and IPv6
	ip = ip.To16()
	return big.NewInt(0).SetBytes(ip)
}
