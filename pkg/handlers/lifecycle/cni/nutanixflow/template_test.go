// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/test/builder"
)

var valuesTemplateFile = func() string {
	dir, err := moduleRootDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(
		dir,
		"charts",
		"cluster-api-runtime-extensions-nutanix",
		"addons",
		"cni",
		"nutanix-flow",
		"values-template.yaml",
	)
}()

func readValuesTemplateFromProjectHelmChart(t *testing.T) string {
	t.Helper()
	bs, err := os.ReadFile(valuesTemplateFile)
	require.NoError(t, err)
	return string(bs)
}

func Test_templateValues(t *testing.T) {
	valuesTemplate := readValuesTemplateFromProjectHelmChart(t)

	tests := []struct {
		name                string
		imagePullSecretName string
		expected            string
	}{
		{
			name:                "without image pull secret",
			imagePullSecretName: "",
			expected: `nutanix-core-flow-ovn-kubernetes:
  k8sAPIServer: "https://192.168.1.100:6443"
  podNetwork: "10.244.0.0/16"
  serviceNetwork: "10.96.0.0/12"
`,
		},
		{
			name:                "with image pull secret",
			imagePullSecretName: "flow-cni-image-pull-secret",
			expected: `nutanix-core-flow-ovn-kubernetes:
  k8sAPIServer: "https://192.168.1.100:6443"
  podNetwork: "10.244.0.0/16"
  serviceNetwork: "10.96.0.0/12"
global:
  imagePullSecretName: "flow-cni-image-pull-secret"
imagePullSecrets:
  - name: "flow-cni-image-pull-secret"
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := createTestCluster(t)
			got, err := templateValues(cluster, valuesTemplate, tt.imagePullSecretName)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func createTestCluster(t *testing.T) *clusterv1beta2.Cluster {
	t.Helper()
	cluster := builder.Cluster("test-namespace", "test-cluster").Build()
	cluster.Spec.ControlPlaneEndpoint = clusterv1beta2.APIEndpoint{
		Host: "192.168.1.100",
		Port: 6443,
	}
	cluster.Spec.ClusterNetwork = clusterv1beta2.ClusterNetwork{
		Pods: clusterv1beta2.NetworkRanges{
			CIDRBlocks: []string{"10.244.0.0/16"},
		},
		Services: clusterv1beta2.NetworkRanges{
			CIDRBlocks: []string{"10.96.0.0/12"},
		},
	}
	return cluster
}

func moduleRootDir() (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{ .Dir }}")
	out, err := cmd.CombinedOutput()
	if err != nil || len(out) == 0 {
		return "", fmt.Errorf("cmd.Dir=%q, cmd.Env=%q, cmd.Args=%q, err=%q, output=%q",
			cmd.Dir,
			cmd.Env,
			cmd.Args,
			err,
			out)
	}
	dir, _, _ := strings.Cut(string(out), "\n")
	return dir, nil
}
