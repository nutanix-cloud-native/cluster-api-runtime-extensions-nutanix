// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/yaml"
)

func TestComputeReservationsScript(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "compute-reservations.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(computeReservationsScript), 0o755))

	tests := []struct {
		name       string
		cores      int
		memKiB     int
		wantCPU    string
		wantMemory string
	}{
		{"1 core 512Mi", 1, 512 * 1024, "60m", "255Mi"},
		{"2 cores 8Gi", 2, 8 * 1024 * 1024, "70m", "1843Mi"},
		{"8 cores 16Gi", 8, 16 * 1024 * 1024, "90m", "2662Mi"},
		{"16 cores 64Gi", 16, 64 * 1024 * 1024, "110m", "5611Mi"},
		{"128 cores 512Gi", 128, 512 * 1024 * 1024, "390m", "17407Mi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir := t.TempDir()
			cmd := exec.Command("/bin/sh", scriptPath)
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("CAREN_NODE_CPU_CORES=%d", tt.cores),
				fmt.Sprintf("CAREN_NODE_MEMORY_KIB=%d", tt.memKiB),
				"CAREN_KUBELET_PATCH_DIR="+outDir,
			)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))

			content, err := os.ReadFile(
				filepath.Join(outDir, "kubeletconfiguration50+strategic.json"),
			)
			require.NoError(t, err)

			var kc kubeletconfigv1beta1.KubeletConfiguration
			require.NoError(t, yaml.Unmarshal(content, &kc))
			assert.Equal(t, tt.wantCPU, kc.KubeReserved["cpu"])
			assert.Equal(t, tt.wantMemory, kc.KubeReserved["memory"])
			assert.Equal(t, "100Mi", kc.EvictionHard["memory.available"])
		})
	}
}
