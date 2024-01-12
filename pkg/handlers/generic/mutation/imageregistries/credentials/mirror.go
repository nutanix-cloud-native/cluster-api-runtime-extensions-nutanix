// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
)

const (
	mirrorCACertPathOnRemote                = "/etc/certs/mirror.pem"
	defaultRegistryMirrorConfigPathOnRemote = "/etc/containerd/certs.d/_default/hosts.toml"
	secretKeyForMirrorCACert                = "ca.crt"
)

//go:embed templates/hosts.toml.gotmpl
var defaultRegistryMirrorPatch []byte

// Default Mirror for all registries. Use a mirror regardless of the intended registry.
// The upstream registry will be automatically used after all defined mirrors have been tried.
// reference: https://github.com/containerd/containerd/blob/main/docs/hosts.md#setup-default-mirror-for-all-registries
func generateDefaultRegistryMirrorFile(config providerConfig) ([]cabpkv1.File, error) {
	t, err := template.New("").Parse(string(defaultRegistryMirrorPatch))
	if err != nil {
		return nil, fmt.Errorf("fail to parse go template for registry mirror: %w", err)
	}
	templateInput := struct {
		URL        string
		CACertPath string
	}{
		URL: config.URL,
	}
	// CA cert is optional for mirror registry.
	// i.e. registry is using signed certificates. Insecure registry will not be allowed.
	if config.CACert != "" {
		templateInput.CACertPath = mirrorCACertPathOnRemote
	}

	var b bytes.Buffer
	err = t.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("failed executing template for registry mirror: %w", err)
	}
	return []cabpkv1.File{
		{
			Path:        defaultRegistryMirrorConfigPathOnRemote,
			Content:     b.String(),
			Permissions: "0600",
		},
	}, nil
}

func generateMirrorCACertFile(
	config providerConfig,
	registry v1alpha1.ImageRegistry,
) []cabpkv1.File {
	if config.CACert == "" {
		return nil
	}
	return []cabpkv1.File{
		{
			Path:        mirrorCACertPathOnRemote,
			Permissions: "0600",
			ContentFrom: &cabpkv1.FileSource{
				Secret: cabpkv1.SecretFileSource{
					Name: registry.CredentialsSecret.Name,
					Key:  secretKeyForMirrorCACert,
				},
			},
		},
	}
}
