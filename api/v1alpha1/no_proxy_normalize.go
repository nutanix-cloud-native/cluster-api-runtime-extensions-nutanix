// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"net/netip"
	"strings"

	"go4.org/netipx"
)

// NormalizeNoProxyEntries ensures that every CIDR and IP range entry in the no_proxy list
// is represented in both notations, so that runtimes supporting either format (e.g. Go
// supports CIDR, Node.js supports ranges) will all correctly bypass the proxy.
//
// For CIDR entries: adds the equivalent IP range (or plain IP for /32 and /128).
// For IP range entries: adds the minimal set of equivalent CIDRs.
// Plain IPs, hostnames, and domains are left unchanged.
// The returned list is deduplicated while preserving order.
func NormalizeNoProxyEntries(entries []string) []string {
	seen := make(map[string]struct{}, len(entries))
	result := make([]string, 0, len(entries))

	add := func(s string) {
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		result = append(result, s)
	}

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if expanded, ok := expandCIDR(entry); ok {
			add(entry)
			for _, e := range expanded {
				add(e)
			}
			continue
		}

		if expanded, ok := expandIPRange(entry); ok {
			add(entry)
			for _, e := range expanded {
				add(e)
			}
			continue
		}

		add(entry)
	}

	return result
}

// expandCIDR attempts to parse the entry as a CIDR prefix. If successful, it returns the
// additional entries to add: a plain IP for single-host prefixes, or an IP range for
// multi-host prefixes.
func expandCIDR(entry string) ([]string, bool) {
	prefix, err := netip.ParsePrefix(entry)
	if err != nil {
		return nil, false
	}

	r := netipx.RangeOfPrefix(prefix)
	if r.From() == r.To() {
		return []string{r.From().String()}, true
	}

	return []string{r.From().String() + "-" + r.To().String()}, true
}

// expandIPRange attempts to parse the entry as an IP range (e.g. "1.2.3.4-1.2.3.5").
// If successful, it returns the minimal set of CIDR prefixes covering the range.
func expandIPRange(entry string) ([]string, bool) {
	from, to, ok := strings.Cut(entry, "-")
	if !ok {
		return nil, false
	}

	fromAddr, err := netip.ParseAddr(strings.TrimSpace(from))
	if err != nil {
		return nil, false
	}

	toAddr, err := netip.ParseAddr(strings.TrimSpace(to))
	if err != nil {
		return nil, false
	}

	r := netipx.IPRangeFrom(fromAddr, toAddr)
	if !r.IsValid() {
		return nil, false
	}

	prefixes := r.Prefixes()
	result := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		result = append(result, p.String())
	}

	return result, true
}
