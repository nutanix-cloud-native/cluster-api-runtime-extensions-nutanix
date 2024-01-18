// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

const (
	//nolint:gosec // Does not contain hard coded credentials.
	installKubeletCredentialProvidersScriptOnRemote = "/etc/konvoy/install-kubelet-credential-providers.sh"

	installKubeletCredentialProvidersScriptOnRemoteCommand = "/bin/bash " + installKubeletCredentialProvidersScriptOnRemote

	//nolint:gosec // Does not contain hard coded credentials.
	dynamicCredentialProviderImage = "ghcr.io/mesosphere/dynamic-credential-provider:v0.5.0"

	//nolint:gosec // Does not contain hard coded credentials.
	credentialProviderTargetDir = "/etc/kubernetes/image-credential-provider/"
)

//go:embed templates/install-kubelet-credential-providers.sh.gotmpl
var installKubeletCredentialProvidersScript []byte

func templateFilesAndCommandsForInstallKubeletCredentialProviders() ([]cabpkv1.File, []string, error) {
	var files []cabpkv1.File
	var commands []string

	installKCPScriptFile, installKCPScriptCommand, err := templateInstallKubeletCredentialProviders()
	if err != nil {
		return nil, nil, err
	}
	if installKCPScriptFile != nil {
		files = append(files, *installKCPScriptFile)
		commands = append(commands, installKCPScriptCommand)
	}

	return files, commands, nil
}

func templateInstallKubeletCredentialProviders() (*cabpkv1.File, string, error) {
	t, err := template.New("").Parse(string(installKubeletCredentialProvidersScript))
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse go template: %w", err)
	}

	templateInput := struct {
		DynamicCredentialProviderImage string
		CredentialProviderTargetDir    string
	}{
		DynamicCredentialProviderImage: dynamicCredentialProviderImage,
		CredentialProviderTargetDir:    credentialProviderTargetDir,
	}

	var b bytes.Buffer
	err = t.Execute(&b, templateInput)
	if err != nil {
		return nil, "", fmt.Errorf("failed executing template: %w", err)
	}

	return &cabpkv1.File{
		Path:        installKubeletCredentialProvidersScriptOnRemote,
		Content:     b.String(),
		Permissions: "0700",
	}, installKubeletCredentialProvidersScriptOnRemoteCommand, nil
}
