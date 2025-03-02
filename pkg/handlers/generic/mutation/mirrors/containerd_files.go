// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
	"text/template"

	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

const (
	containerdHostsConfigurationOnRemote = "/etc/containerd/certs.d/_default/hosts.toml"
	secretKeyForCACert                   = "ca.crt"
)

var (
	//go:embed templates/hosts.toml.gotmpl
	containerdHostsConfiguration []byte

	containerdDefaultHostsConfigurationTemplate = template.Must(
		template.New("").Parse(string(containerdHostsConfiguration)),
	)

	//go:embed templates/containerd-registry-config-drop-in.toml
	containerdRegistryConfigDropIn             []byte
	containerdRegistryConfigDropInFileOnRemote = common.ContainerdPatchPathOnRemote(
		"registry-config.toml",
	)

	caCertPathOnRemoteFmt = "/etc/containerd/certs.d/%s/ca.crt"
)

type containerdConfig struct {
	URL          string
	CASecretName string
	CACert       string
	Mirror       bool
}

// fileNameFromURL returns a file name for a registry URL.
// Follows a convention of replacing all non-alphanumeric characters with "-".
func (c containerdConfig) filePathFromURL() (string, error) {
	registryURL, err := url.ParseRequestURI(c.URL)
	if err != nil {
		return "", fmt.Errorf("failed parsing registry URL: %w", err)
	}

	return fmt.Sprintf(caCertPathOnRemoteFmt, registryURL.Host), nil
}

// Return true if configuration is a mirror or has a CA certificate.
func (c containerdConfig) needContainerdConfiguration() bool {
	return c.CACert != "" || c.Mirror
}

// Containerd registry configuration created at /etc/containerd/certs.d/_default/hosts.toml for:
//
//  1. Setting the default mirror for all registries.
//     The upstream registry will be automatically used after all defined mirrors have been tried.
//     https://github.com/containerd/containerd/blob/main/docs/hosts.md#setup-default-mirror-for-all-registries
//
//  2. Setting CA certificate for global image registry mirror.
func generateContainerdDefaultHostsFile(
	configs []containerdConfig,
) (*cabpkv1.File, error) {
	if len(configs) == 0 {
		return nil, nil
	}

	type templateInput struct {
		URL        string
		CACertPath string
	}

	inputs := make([]templateInput, 0, len(configs))

	for _, config := range configs {
		if !config.Mirror {
			continue
		}

		formattedURL, err := formatURLForContainerd(config.URL)
		if err != nil {
			return nil, fmt.Errorf("failed formatting image registry URL for Containerd: %w", err)
		}

		input := templateInput{
			URL: formattedURL,
		}
		// CA cert is optional for mirror registry.
		// i.e. registry is using signed certificates. Insecure registry will not be allowed.
		if config.CACert != "" {
			registryCACertPathOnRemote, err := config.filePathFromURL()
			if err != nil {
				return nil, fmt.Errorf(
					"failed generating CA certificate file path from URL: %w",
					err,
				)
			}
			input.CACertPath = registryCACertPathOnRemote
		}

		inputs = append(inputs, input)
	}

	// No need to generate the file if there are no mirrors.
	if len(inputs) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	err := containerdDefaultHostsConfigurationTemplate.Execute(&b, inputs)
	if err != nil {
		return nil, fmt.Errorf("failed executing template for Containerd hosts.toml file: %w", err)
	}
	return &cabpkv1.File{
		Path: containerdHostsConfigurationOnRemote,
		// Trimming the leading and trailing whitespaces in the template did not work as expected with multiple configs.
		Content:     fmt.Sprintf("%s\n", strings.TrimSpace(b.String())),
		Permissions: "0600",
	}, nil
}

func generateRegistryCACertFiles(
	configs []containerdConfig,
) ([]cabpkv1.File, error) {
	if len(configs) == 0 {
		return nil, nil
	}

	var files []cabpkv1.File //nolint:prealloc // We don't know the size of the slice yet.

	filesToGenerate, err := registryCACertFiles(configs)
	if err != nil {
		return nil, err
	}
	for _, file := range filesToGenerate {
		files = append(files, cabpkv1.File{
			Path:        file.path,
			Permissions: "0600",
			ContentFrom: &cabpkv1.FileSource{
				Secret: cabpkv1.SecretFileSource{
					Name: file.caSecretName,
					Key:  secretKeyForCACert,
				},
			},
		})
	}

	return files, nil
}

func generateContainerdRegistryConfigDropInFile() []cabpkv1.File {
	return []cabpkv1.File{
		{
			Path:        containerdRegistryConfigDropInFileOnRemote,
			Content:     string(containerdRegistryConfigDropIn),
			Permissions: "0600",
		},
	}
}

func formatURLForContainerd(uri string) (string, error) {
	mirrorURL, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", fmt.Errorf("failed parsing mirror: %w", err)
	}

	mirror := fmt.Sprintf("%s://%s", mirrorURL.Scheme, mirrorURL.Host)
	// assume Containerd expects the following pattern:
	//   scheme://host/v2/path
	mirrorPath := "v2"
	if mirrorURL.Path != "" {
		mirrorPath = path.Join(mirrorPath, mirrorURL.Path)
	}
	// using path.Join on all elements incorrectly drops a "/" from "https://"
	return fmt.Sprintf("%s/%s", mirror, mirrorPath), nil
}

type containerdConfigFile struct {
	path         string
	url          string
	caSecretName string
	caCert       string
}

// registryCACertFiles returns a list of CA certificate files
// that should be generated for the given containerd configurations.
// If any of the provided configurations share the same url.Host only a single file will be generated.
// An error will be returned, if the CA certificate content for the same URL.Host do not match.
func registryCACertFiles(configs []containerdConfig) ([]containerdConfigFile, error) {
	filesToGenerate := make([]containerdConfigFile, 0)
	for _, config := range configs {
		// Skip if CA certificate is not provided.
		if config.CASecretName == "" {
			continue
		}
		registryCACertPathOnRemote, err := config.filePathFromURL()
		if err != nil {
			return nil, fmt.Errorf("failed generating CA certificate file path from URL: %w", err)
		}

		foundIndex := slices.IndexFunc(filesToGenerate, func(f containerdConfigFile) bool {
			return registryCACertPathOnRemote == f.path
		})
		// File not already found and needs to be generated.
		if foundIndex == -1 {
			filesToGenerate = append(filesToGenerate, containerdConfigFile{
				path:         registryCACertPathOnRemote,
				url:          config.URL,
				caSecretName: config.CASecretName,
				caCert:       config.CACert,
			})
			continue
		}
		// File is already in the list, check if the CA certificate content matches.
		if config.CACert != filesToGenerate[foundIndex].caCert {
			return nil, fmt.Errorf(
				"CA certificate content for %q does not match one for %q",
				config.URL,
				filesToGenerate[foundIndex].url,
			)
		}
	}

	return filesToGenerate, nil
}
