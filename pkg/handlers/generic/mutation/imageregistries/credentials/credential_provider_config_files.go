// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"path"
	"text/template"

	credentialproviderv1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/imageregistries/credentials/credentialprovider"
)

const (
	//nolint:gosec // Does not contain hard coded credentials.
	kubeletStaticCredentialProviderCredentialsOnRemote = "/etc/kubernetes/static-image-credentials.json"

	//nolint:gosec // Does not contain hard coded credentials.
	kubeletImageCredentialProviderConfigOnRemote = "/etc/kubernetes/image-credential-provider-config.yaml"

	//nolint:gosec // Does not contain hard coded credentials.
	kubeletDynamicCredentialProviderConfigOnRemote = "/etc/kubernetes/dynamic-credential-provider-config.yaml"

	azureCloudConfigFilePath = "/etc/kubernetes/azure.json"
)

var (
	//go:embed templates/dynamic-credential-provider-config.yaml.gotmpl
	dynamicCredentialProviderConfigPatch []byte

	dynamicCredentialProviderConfigPatchTemplate = template.Must(
		template.New("").Parse(string(dynamicCredentialProviderConfigPatch)),
	)

	//go:embed templates/kubelet-image-credential-provider-config.yaml.gotmpl
	kubeletImageCredentialProviderConfigPatch []byte

	kubeletImageCredentialProviderConfigPatchTemplate = template.Must(
		template.New("").Parse(string(kubeletImageCredentialProviderConfigPatch)),
	)
)

type providerConfig struct {
	URL      string
	Username string
	Password string
	Mirror   bool
}

func (c providerConfig) isCredentialsEmpty() bool {
	return c.Username == "" &&
		c.Password == ""
}

func (c providerConfig) requiresStaticCredentials() (bool, error) {
	registryHostWithPath, err := c.registryHostWithPath()
	if err != nil {
		return false, fmt.Errorf(
			"failed to get registry host with path: %w",
			err,
		)
	}

	knownRegistryProvider, err := credentialprovider.URLMatchesKnownRegistryProvider(
		registryHostWithPath,
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to check if registry matches a known registry provider: %w",
			err,
		)
	}

	// require static credentials if the registry provider is not known
	return !knownRegistryProvider, nil
}

func (c providerConfig) registryHostWithPath() (string, error) {
	registryURL, err := url.ParseRequestURI(c.URL)
	if err != nil {
		return "", fmt.Errorf("failed parsing registry URL: %w", err)
	}

	registryHostWithPath := registryURL.Host
	if registryURL.Path != "" {
		registryHostWithPath = path.Join(registryURL.Host, registryURL.Path)
	}

	return registryHostWithPath, nil
}

func templateFilesForImageCredentialProviderConfigs(
	configs []providerConfig,
) ([]cabpkv1.File, error) {
	var files []cabpkv1.File

	kubeletCredentialProviderConfigFile, err := templateKubeletCredentialProviderConfig(configs)
	if err != nil {
		return nil, err
	}
	if kubeletCredentialProviderConfigFile != nil {
		files = append(files, *kubeletCredentialProviderConfigFile)
	}

	kubeletDynamicCredentialProviderConfigFile, err := templateDynamicCredentialProviderConfig(
		configs,
	)
	if err != nil {
		return nil, err
	}
	if kubeletDynamicCredentialProviderConfigFile != nil {
		files = append(files, *kubeletDynamicCredentialProviderConfigFile)
	}

	return files, nil
}

func templateKubeletCredentialProviderConfig(
	configs []providerConfig,
) (*cabpkv1.File, error) {
	providerBinary, providerArgs, providerAPIVersion := kubeletCredentialProvider()

	// In addition to the globs already defined in the template, also include the user provided registries.
	//
	// This is needed to match registries with a port and/or a URL path.
	// From https://kubernetes.io/docs/tasks/administer-cluster/kubelet-credential-provider/#configure-image-matching
	registryHosts := make([]string, 0, len(configs))
	for _, config := range configs {
		registryHostWithPath, err := config.registryHostWithPath()
		if err != nil {
			return nil, err
		}
		registryHosts = append(registryHosts, registryHostWithPath)
	}

	templateInput := struct {
		RegistryHosts      []string
		ProviderBinary     string
		ProviderArgs       []string
		ProviderAPIVersion string
	}{
		RegistryHosts:      registryHosts,
		ProviderBinary:     providerBinary,
		ProviderArgs:       providerArgs,
		ProviderAPIVersion: providerAPIVersion,
	}

	return fileFromTemplate(
		kubeletImageCredentialProviderConfigPatchTemplate,
		templateInput,
		kubeletImageCredentialProviderConfigOnRemote,
	)
}

func templateDynamicCredentialProviderConfig(
	configs []providerConfig,
) (*cabpkv1.File, error) {
	type templateInput struct {
		RegistryHost       string
		ProviderBinary     string
		ProviderArgs       []string
		ProviderAPIVersion string
		Mirror             bool
	}

	inputs := make([]templateInput, 0, len(configs))

	for _, config := range configs {
		registryHostWithPath, err := config.registryHostWithPath()
		if err != nil {
			return nil, err
		}

		providerBinary, providerArgs, providerAPIVersion, err := dynamicCredentialProvider(
			registryHostWithPath,
		)
		if err != nil {
			return nil, err
		}

		inputs = append(inputs, templateInput{
			RegistryHost:       registryHostWithPath,
			ProviderBinary:     providerBinary,
			ProviderArgs:       providerArgs,
			ProviderAPIVersion: providerAPIVersion,
			Mirror:             config.Mirror,
		})
	}

	return fileFromTemplate(
		dynamicCredentialProviderConfigPatchTemplate,
		inputs,
		kubeletDynamicCredentialProviderConfigOnRemote,
	)
}

func kubeletCredentialProvider() (providerBinary string, providerArgs []string, providerAPIVersion string) {
	return "dynamic-credential-provider",
		[]string{"get-credentials", "-c", kubeletDynamicCredentialProviderConfigOnRemote},
		credentialproviderv1.SchemeGroupVersion.String()
}

func dynamicCredentialProvider(host string) (
	providerBinary string, providerArgs []string, providerAPIVersion string, err error,
) {
	if matches, err := credentialprovider.URLMatchesECR(host); matches || err != nil {
		return "ecr-credential-provider", []string{"get-credentials"},
			credentialproviderv1.SchemeGroupVersion.String(), err
	}

	if matches, err := credentialprovider.URLMatchesGCR(host); matches || err != nil {
		return "gcr-credential-provider", []string{"get-credentials"},
			credentialproviderv1.SchemeGroupVersion.String(), err
	}

	if matches, err := credentialprovider.URLMatchesACR(host); matches || err != nil {
		return "acr-credential-provider", []string{
			azureCloudConfigFilePath,
		}, credentialproviderv1.SchemeGroupVersion.String(), err
	}

	// if no supported provider was found, assume we are using the static credential provider
	return "static-credential-provider",
		[]string{kubeletStaticCredentialProviderCredentialsOnRemote},
		credentialproviderv1.SchemeGroupVersion.String(),
		nil
}

func fileFromTemplate(
	t *template.Template,
	templateInput any,
	fPath string,
) (*cabpkv1.File, error) {
	var b bytes.Buffer
	err := t.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("failed executing template: %w", err)
	}

	return &cabpkv1.File{
		Path:        fPath,
		Content:     b.String(),
		Permissions: "0600",
	}, nil
}
