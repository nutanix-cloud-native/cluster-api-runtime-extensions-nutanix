// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package containerdapplypatchesandrestart

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

const (
	tomlMergeImage = "ghcr.io/mesosphere/toml-merge:v0.2.0"
)

var (
	containerdApplyPatchesScriptOnRemote = common.ContainerdScriptPathOnRemote(
		"apply-patches.sh",
	)
	containerdApplyPatchesScriptOnRemoteCommand = "/bin/bash " + containerdApplyPatchesScriptOnRemote
)

//go:embed templates/containerd-apply-patches.sh.gotmpl
var containerdApplyConfigPatchesScript []byte

func generateContainerdApplyPatchesScript() (bootstrapv1.File, string, error) {
	t, err := template.New("").Parse(string(containerdApplyConfigPatchesScript))
	if err != nil {
		return bootstrapv1.File{}, "", fmt.Errorf("failed to parse go template: %w", err)
	}

	templateInput := struct {
		TOMLMergeImage string
		PatchDir       string
	}{
		TOMLMergeImage: tomlMergeImage,
		PatchDir:       common.ContainerdPatchDirOnRemote,
	}

	var b bytes.Buffer
	err = t.Execute(&b, templateInput)
	if err != nil {
		return bootstrapv1.File{}, "", fmt.Errorf("failed executing template: %w", err)
	}

	return bootstrapv1.File{
		Path:        containerdApplyPatchesScriptOnRemote,
		Content:     b.String(),
		Permissions: "0700",
	}, containerdApplyPatchesScriptOnRemoteCommand, nil
}
