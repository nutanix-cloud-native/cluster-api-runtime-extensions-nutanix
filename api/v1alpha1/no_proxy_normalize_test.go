// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"testing"

	"github.com/onsi/gomega"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestNormalizeNoProxyEntries(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{{
		name:     "nil input",
		input:    nil,
		expected: []string{},
	}, {
		name:     "empty input",
		input:    []string{},
		expected: []string{},
	}, {
		name:     "plain IP unchanged",
		input:    []string{"10.0.0.1"},
		expected: []string{"10.0.0.1"},
	}, {
		name:     "hostname unchanged",
		input:    []string{"example.com"},
		expected: []string{"example.com"},
	}, {
		name:     "domain with leading dot unchanged",
		input:    []string{".svc.cluster.local"},
		expected: []string{".svc.cluster.local"},
	}, {
		name:  "/32 CIDR expands to plain IP",
		input: []string{"10.0.0.1/32"},
		expected: []string{
			"10.0.0.1/32",
			"10.0.0.1",
		},
	}, {
		name:  "/128 IPv6 CIDR expands to plain IP",
		input: []string{"fd00::1/128"},
		expected: []string{
			"fd00::1/128",
			"fd00::1",
		},
	}, {
		name:  "/24 CIDR expands to range",
		input: []string{"10.0.0.0/24"},
		expected: []string{
			"10.0.0.0/24",
			"10.0.0.0-10.0.0.255",
		},
	}, {
		name:  "/16 CIDR expands to range",
		input: []string{"172.16.0.0/16"},
		expected: []string{
			"172.16.0.0/16",
			"172.16.0.0-172.16.255.255",
		},
	}, {
		name:  "/31 CIDR expands to two-IP range",
		input: []string{"10.0.0.4/31"},
		expected: []string{
			"10.0.0.4/31",
			"10.0.0.4-10.0.0.5",
		},
	}, {
		name:  "IP range mapping to single CIDR",
		input: []string{"10.0.0.0-10.0.0.255"},
		expected: []string{
			"10.0.0.0-10.0.0.255",
			"10.0.0.0/24",
		},
	}, {
		name:  "IP range decomposing into multiple CIDRs",
		input: []string{"10.0.0.0-10.0.0.2"},
		expected: []string{
			"10.0.0.0-10.0.0.2",
			"10.0.0.0/31",
			"10.0.0.2/32",
		},
	}, {
		name:  "single IP range expands to CIDR",
		input: []string{"10.0.0.1-10.0.0.1"},
		expected: []string{
			"10.0.0.1-10.0.0.1",
			"10.0.0.1/32",
		},
	}, {
		name:  "IPv6 CIDR expands to range",
		input: []string{"fd00::/120"},
		expected: []string{
			"fd00::/120",
			"fd00::-fd00::ff",
		},
	}, {
		name:  "IPv6 range expands to CIDR",
		input: []string{"fd00::0-fd00::ff"},
		expected: []string{
			"fd00::0-fd00::ff",
			"fd00::/120",
		},
	}, {
		name: "mixed entries",
		input: []string{
			"localhost",
			"127.0.0.1",
			"10.0.0.0/24",
			"1.2.3.4-1.2.3.5",
			".svc.cluster.local",
			"example.com",
		},
		expected: []string{
			"localhost",
			"127.0.0.1",
			"10.0.0.0/24",
			"10.0.0.0-10.0.0.255",
			"1.2.3.4-1.2.3.5",
			"1.2.3.4/31",
			".svc.cluster.local",
			"example.com",
		},
	}, {
		name: "deduplicates equivalent entries",
		input: []string{
			"10.0.0.0/24",
			"10.0.0.0-10.0.0.255",
		},
		expected: []string{
			"10.0.0.0/24",
			"10.0.0.0-10.0.0.255",
		},
	}, {
		name: "deduplicates /32 CIDR with existing plain IP",
		input: []string{
			"10.0.0.1",
			"10.0.0.1/32",
		},
		expected: []string{
			"10.0.0.1",
			"10.0.0.1/32",
		},
	}, {
		name:     "empty strings are skipped",
		input:    []string{"", "  ", "10.0.0.1"},
		expected: []string{"10.0.0.1"},
	}, {
		name:  "whitespace is trimmed",
		input: []string{" 10.0.0.0/24 "},
		expected: []string{
			"10.0.0.0/24",
			"10.0.0.0-10.0.0.255",
		},
	}}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)
			g.Expect(v1alpha1.NormalizeNoProxyEntries(tt.input)).To(gomega.Equal(tt.expected))
		})
	}
}
