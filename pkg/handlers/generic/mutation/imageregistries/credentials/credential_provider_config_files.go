// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"path"
	"text/template"

	credentialproviderv1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries/credentials/credentialprovider"
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

	//go:embed templates/kubelet-image-credential-provider-config.yaml.gotmpl
	kubeletImageCredentialProviderConfigPatch []byte
)

var ErrCredentialsNotFound = errors.New("registry credentials not found")

type providerConfig struct {
	URL      string
	Username string
	Password string
}

func (c providerConfig) isCredentialsEmpty() bool {
	return c.Username == "" &&
		c.Password == ""
}

func templateFilesForImageCredentialProviderConfigs(config providerConfig) ([]cabpkv1.File, error) {
	var files []cabpkv1.File

	kubeletCredentialProviderConfigFile, err := templateKubeletCredentialProviderConfig()
	if err != nil {
		return nil, err
	}
	if kubeletCredentialProviderConfigFile != nil {
		files = append(files, *kubeletCredentialProviderConfigFile)
	}

	kubeletDynamicCredentialProviderConfigFile, err := templateDynamicCredentialProviderConfig(
		config,
	)
	if err != nil {
		return nil, err
	}
	if kubeletDynamicCredentialProviderConfigFile != nil {
		files = append(files, *kubeletDynamicCredentialProviderConfigFile)
	}

	return files, nil
}

func templateKubeletCredentialProviderConfig() (*cabpkv1.File, error) {
	t := template.New("")
	t, err := t.Parse(string(kubeletImageCredentialProviderConfigPatch))
	if err != nil {
		return nil, fmt.Errorf("failed to parse go template: %w", err)
	}

	providerBinary, providerArgs, providerAPIVersion := kubeletCredentialProvider()

	templateInput := struct {
		ProviderBinary     string
		ProviderArgs       []string
		ProviderAPIVersion string
	}{
		ProviderBinary:     providerBinary,
		ProviderArgs:       providerArgs,
		ProviderAPIVersion: providerAPIVersion,
	}

	return fileFromTemplate(t, templateInput, kubeletImageCredentialProviderConfigOnRemote)
}

func templateDynamicCredentialProviderConfig(
	config providerConfig,
) (*cabpkv1.File, error) {
	registryURL, err := url.ParseRequestURI(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed parsing registry URL: %w", err)
	}

	t := template.New("")
	t, err = t.Parse(string(dynamicCredentialProviderConfigPatch))
	if err != nil {
		return nil, fmt.Errorf("failed to parse go template: %w", err)
	}

	registryHostWithPath := registryURL.Host
	if registryURL.Path != "" {
		registryHostWithPath = path.Join(registryURL.Host, registryURL.Path)
	}

	supportedProvider, err := credentialprovider.URLMatchesSupportedProvider(registryHostWithPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if registry matches a supporterd provider: %w", err)
	}
	if config.isCredentialsEmpty() && !supportedProvider {
		return nil, ErrCredentialsNotFound
	}

	providerBinary, providerArgs, providerAPIVersion, err := dynamicCredentialProvider(
		registryHostWithPath,
	)
	if err != nil {
		return nil, err
	}

	templateInput := struct {
		RegistryHost       string
		ProviderBinary     string
		ProviderArgs       []string
		ProviderAPIVersion string
	}{
		RegistryHost:       registryHostWithPath,
		ProviderBinary:     providerBinary,
		ProviderArgs:       providerArgs,
		ProviderAPIVersion: providerAPIVersion,
	}

	return fileFromTemplate(t, templateInput, kubeletDynamicCredentialProviderConfigOnRemote)
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
