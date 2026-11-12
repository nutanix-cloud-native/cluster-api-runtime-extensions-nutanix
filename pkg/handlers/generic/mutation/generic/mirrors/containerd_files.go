// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"text/template"

	"github.com/samber/lo"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"

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

	//go:embed templates/registry-hosts.toml.gotmpl
	containerdRegistryHostsConfiguration []byte

	containerdRegistryHostsConfigurationTemplate = template.Must(
		template.New("").Parse(string(containerdRegistryHostsConfiguration)),
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

func (c containerdConfig) isHTTP() bool {
	registryURL, err := url.ParseRequestURI(c.URL)
	if err != nil {
		return false
	}
	return registryURL.Scheme == "http"
}

// Return true if configuration is a mirror, has a CA certificate, or uses http scheme.
func (c containerdConfig) needContainerdConfiguration() bool {
	return c.CACert != "" || c.Mirror || c.isHTTP()
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
) (*bootstrapv1.File, error) {
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
	return &bootstrapv1.File{
		Path: containerdHostsConfigurationOnRemote,
		// Trimming the leading and trailing whitespaces in the template did not work as expected with multiple configs.
		Content:     fmt.Sprintf("%s\n", strings.TrimSpace(b.String())),
		Permissions: "0600",
	}, nil
}

func generateContainerdPerHostHostsFiles(
	configs []containerdConfig,
) ([]bootstrapv1.File, error) {
	if len(configs) == 0 {
		return nil, nil
	}

	type perHostEntry struct {
		host    string
		hostURL string
	}

	// Validate that the same host is not used with different schemes.
	// Containerd's per-host namespace is keyed by host:port, so mixed http/https
	// for the same host would be ambiguous (hosts.toml can only define one transport).
	hostSchemes := make(map[string]string)
	for _, config := range configs {
		if config.Mirror {
			continue
		}
		registryURL, err := url.ParseRequestURI(config.URL)
		if err != nil {
			return nil, fmt.Errorf("failed parsing registry URL: %w", err)
		}
		host := registryURL.Host
		scheme := registryURL.Scheme
		if existing, ok := hostSchemes[host]; ok && existing != scheme {
			return nil, fmt.Errorf(
				"%w: %q (schemes %q and %q)",
				ErrConflictingRegistrySchemes,
				host,
				existing,
				scheme,
			)
		}
		hostSchemes[host] = scheme
	}

	// Deduplicate by host.
	hosts := make(map[string]perHostEntry)
	for _, config := range configs {
		if !config.isHTTP() || config.Mirror {
			continue
		}

		registryURL, err := url.ParseRequestURI(config.URL)
		if err != nil {
			return nil, fmt.Errorf("failed parsing registry URL: %w", err)
		}

		host := registryURL.Host
		if _, exists := hosts[host]; exists {
			continue
		}

		// For per-registry http, the repository path is not part of the host
		// configuration; enabling http for the host covers all paths.
		hostURL := fmt.Sprintf("%s://%s", registryURL.Scheme, host)

		hosts[host] = perHostEntry{
			host:    host,
			hostURL: hostURL,
		}
	}

	if len(hosts) == 0 {
		return nil, nil
	}

	var files []bootstrapv1.File
	for _, entry := range hosts {
		type templateInput struct {
			HostURL string
		}

		input := templateInput{
			HostURL: entry.hostURL,
		}

		var b bytes.Buffer
		if err := containerdRegistryHostsConfigurationTemplate.Execute(&b, input); err != nil {
			return nil, fmt.Errorf("failed executing template for per-host hosts.toml: %w", err)
		}

		files = append(files, bootstrapv1.File{
			Path:        fmt.Sprintf("/etc/containerd/certs.d/%s/hosts.toml", entry.host),
			Content:     fmt.Sprintf("%s\n", strings.TrimSpace(b.String())),
			Permissions: "0600",
		})
	}

	return files, nil
}

func generateRegistryCACertFiles(
	configs []containerdConfig,
) ([]bootstrapv1.File, error) {
	if len(configs) == 0 {
		return nil, nil
	}

	var files []bootstrapv1.File

	filesToGenerate, err := registryCACertFiles(configs)
	if err != nil {
		return nil, err
	}
	for _, file := range filesToGenerate {
		files = append(files, bootstrapv1.File{
			Path:        file.path,
			Permissions: "0600",
			ContentFrom: bootstrapv1.FileSource{
				Secret: bootstrapv1.SecretFileSource{
					Name: file.caSecretName,
					Key:  secretKeyForCACert,
				},
			},
		})
	}

	return files, nil
}

func generateContainerdRegistryConfigDropInFile() []bootstrapv1.File {
	return []bootstrapv1.File{
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

var ErrConflictingRegistryCACertificates = errors.New(
	"conflicting CA certificate specified for registry host",
)

var ErrConflictingRegistrySchemes = errors.New(
	"conflicting scheme specified for registry host",
)

// registryCACertFiles returns a list of CA certificate files
// that should be generated for the given containerd configurations.
// If any of the provided configurations share the same url.Host only a single file will be generated.
// An error will be returned, if the CA certificate content for the same URL.Host do not match.
func registryCACertFiles(configs []containerdConfig) ([]containerdConfigFile, error) {
	filesToGenerate := make([]containerdConfigFile, 0, len(configs))

	for _, config := range configs {
		// Skip if CA certificate is not provided.
		if config.CASecretName == "" {
			continue
		}
		registryCACertPathOnRemote, err := config.filePathFromURL()
		if err != nil {
			return nil, fmt.Errorf("failed generating CA certificate file path from URL: %w", err)
		}

		existingFileToGenerate, existing := lo.Find(
			filesToGenerate,
			func(f containerdConfigFile) bool {
				return registryCACertPathOnRemote == f.path
			},
		)
		// File exists and needs to be checked for conflicts.
		if existing {
			if config.CACert != existingFileToGenerate.caCert {
				return nil, fmt.Errorf(
					"%w: %q (from secrets %q and %q)",
					ErrConflictingRegistryCACertificates,
					config.URL,
					config.CASecretName,
					existingFileToGenerate.caSecretName,
				)
			}

			// File already found with matching content - skipping.
			continue
		}

		// File not already found and needs to be generated.
		filesToGenerate = append(filesToGenerate, containerdConfigFile{
			path:         registryCACertPathOnRemote,
			url:          config.URL,
			caSecretName: config.CASecretName,
			caCert:       config.CACert,
		})
	}

	return filesToGenerate, nil
}
