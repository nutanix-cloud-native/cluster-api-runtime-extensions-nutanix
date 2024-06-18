// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"text/template"

	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

const (
	containerdHostsConfigurationOnRemote = "/etc/containerd/certs.d/_default/hosts.toml"
	secretKeyForMirrorCACert             = "ca.crt"
)

var (
	//go:embed templates/hosts.toml.gotmpl
	containerdHostsConfiguration []byte

	containerdHostsConfigurationTemplate = template.Must(
		template.New("").Parse(string(containerdHostsConfiguration)),
	)

	//go:embed templates/containerd-registry-config-drop-in.toml
	containerdRegistryConfigDropIn             []byte
	containerdRegistryConfigDropInFileOnRemote = common.ContainerdPatchPathOnRemote(
		"registry-config.toml",
	)

	mirrorCACertPathOnRemoteFmt = "/etc/certs/%s.pem"
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

	registryHostWithPath := registryURL.Host
	if registryURL.Path != "" {
		registryHostWithPath = path.Join(registryURL.Host, registryURL.Path)
	}

	// replace all non-alphanumeric characters with "-"
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	replaced := re.ReplaceAllString(registryHostWithPath, "-")

	return fmt.Sprintf(mirrorCACertPathOnRemoteFmt, replaced), nil
}

// Return true if configuration is a mirror or has a CA certificate.
func (c containerdConfig) needContainerdConfiguration() bool {
	return c.CACert != "" || c.Mirror
}

// Containerd registry configuration created at /etc/containerd/certs.d/_default/hosts.toml for:
//
//  1. Set the default mirror for all registries.
//     The upstream registry will be automatically used after all defined mirrors have been tried.
//     https://github.com/containerd/containerd/blob/main/docs/hosts.md#setup-default-mirror-for-all-registries
//
//  2. Setting CA certificate for global image registry mirror and image registries.
func generateContainerdHostsFile(
	configs []containerdConfig,
) (*cabpkv1.File, error) {
	if len(configs) == 0 {
		return nil, nil
	}

	type templateInput struct {
		URL        string
		CACertPath string
		Mirror     bool
	}

	inputs := make([]templateInput, 0, len(configs))

	for _, config := range configs {
		if !config.needContainerdConfiguration() {
			continue
		}

		formattedURL, err := formatURLForContainerd(config.URL)
		if err != nil {
			return nil, fmt.Errorf("failed formatting image registry URL for Containerd: %w", err)
		}

		input := templateInput{
			URL:    formattedURL,
			Mirror: config.Mirror,
		}
		// CA cert is optional for mirror registry.
		// i.e. registry is using signed certificates. Insecure registry will not be allowed.
		if config.CACert != "" {
			var registryCACertPathOnRemote string
			registryCACertPathOnRemote, err = config.filePathFromURL()
			if err != nil {
				return nil, fmt.Errorf("failed generating CA certificate file path from URL: %w", err)
			}
			input.CACertPath = registryCACertPathOnRemote
		}

		inputs = append(inputs, input)
	}

	var b bytes.Buffer
	err := containerdHostsConfigurationTemplate.Execute(&b, inputs)
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

	for _, config := range configs {
		if config.CASecretName == "" {
			continue
		}

		registryCACertPathOnRemote, err := config.filePathFromURL()
		if err != nil {
			return nil, fmt.Errorf("failed generating CA certificate file path from URL: %w", err)
		}
		files = append(files, cabpkv1.File{
			Path:        registryCACertPathOnRemote,
			Permissions: "0600",
			ContentFrom: &cabpkv1.FileSource{
				Secret: cabpkv1.SecretFileSource{
					Name: config.CASecretName,
					Key:  secretKeyForMirrorCACert,
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
