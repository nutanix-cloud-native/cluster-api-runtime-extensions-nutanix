// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

func TestHandlersAddFlags_Multus(t *testing.T) {
	t.Parallel()

	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	h := New(options.NewGlobalOptions())
	h.AddFlags(flagSet)

	flag := flagSet.Lookup("cni.multus.helm-addon.default-values-template-configmap-name")
	require.NotNil(
		t,
		flag,
		"expected cni.multus.helm-addon.default-values-template-configmap-name flag to be registered",
	)
	assert.Equal(t, "default-multus-values-template", flag.DefValue)
}

func TestDeploymentTemplate_MultusFlag(t *testing.T) {
	t.Parallel()

	helmBin, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm not found in PATH")
	}

	modRootCmd := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{ .Dir }}")
	out, err := modRootCmd.CombinedOutput()
	require.NoError(t, err)
	repoRoot, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")

	chartDir := filepath.Join(repoRoot, "charts", "cluster-api-runtime-extensions-nutanix")
	cmd := exec.CommandContext(
		t.Context(),
		helmBin,
		"template",
		chartDir,
		"--show-only",
		"templates/deployment.yaml",
	)
	renderedBytes, err := cmd.CombinedOutput()
	require.NoError(t, err, "helm template failed:\n%s", string(renderedBytes))

	assert.Contains(
		t,
		string(renderedBytes),
		"--cni.multus.helm-addon.default-values-template-configmap-name=default-multus-values-template",
	)
}
