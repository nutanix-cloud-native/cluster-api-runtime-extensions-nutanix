// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import "testing"

func TestCleanPCVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Lowercases and trims", "  PC.7.3.0  ", "7.3.0"},
		{"Handles missing prefix", "7.5.1", "7.5.1"},
		{"Handles uppercase PC prefix", "Pc.2024.1.0.1", "2024.1.0.1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CleanPCVersion(tt.input); got != tt.expected {
				t.Fatalf("CleanPCVersion(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestComparePCVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"Equal versions", "7.3.0", "7.3.0", 0},
		{"Greater minor", "7.5.0", "7.3.0", 1},
		{"Less minor", "7.3.0", "7.5.0", -1},
		{"7.x beats 202x", "7.3.0", "2024.1.0.1", 1},
		{"202x loses to 7.x", "2024.1.0.1", "7.3.0", -1},
		{"Handles master as latest", "master", "7.5.0", 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ComparePCVersions(tt.v1, tt.v2); got != tt.expected {
				t.Fatalf("ComparePCVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}
