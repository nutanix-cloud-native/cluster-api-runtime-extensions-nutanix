// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"path"
	"text/template"

	credentialproviderv1alpha1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1alpha1"
	credentialproviderv1beta1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1beta1"
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

	//nolint:gosec // Does not contain hard coded credentials.
	dynamicCredentialProviderImage = "ghcr.io/mesosphere/dynamic-credential-provider:v0.2.0"

	//nolint:gosec // Does not contain hard coded credentials.
	credentialProviderTargetDir = "/etc/kubernetes/image-credential-provider/"
)

var (
	//go:embed templates/dynamic-credential-provider-config.yaml.gotmpl
	dynamicCredentialProviderConfigPatch []byte

	//go:embed templates/static-credential-provider.json.gotmpl
	staticCredentialProviderConfigPatch []byte

	//go:embed templates/image-credential-provider-config.yaml.gotmpl
	imageCredentialProviderConfigPatch []byte

	//go:embed templates/install-kubelet-credential-providers.sh.gotmpl
	installKubeletCredentialProvidersScript []byte
)

type providerInput struct {
	URL      string
	Username string
	Password string
}

func (c providerInput) isCredentialsEmpty() bool {
	return c.Username == "" &&
		c.Password == ""
}

func templateFilesForImageCredentialProviderConfigs(credentials providerInput) ([]cabpkv1.File, error) {
	var files []cabpkv1.File

	kubeletCredentialProviderConfigFile, err := templateKubeletCredentialProviderConfig(credentials)
	if err != nil {
		return nil, err
	}
	if kubeletCredentialProviderConfigFile != nil {
		files = append(files, *kubeletCredentialProviderConfigFile)
	}

	kubeletDynamicCredentialProviderConfigFile, err := templateDynamicCredentialProviderConfig(credentials)
	if err != nil {
		return nil, err
	}
	if kubeletDynamicCredentialProviderConfigFile != nil {
		files = append(files, *kubeletDynamicCredentialProviderConfigFile)
	}

	return files, nil
}

func templateKubeletCredentialProviderConfig(credentials providerInput) (*cabpkv1.File, error) {
	return templateCredentialProviderConfig(
		credentials,
		imageCredentialProviderConfigPatch,
		kubeletImageCredentialProviderConfigOnRemote,
		kubeletCredentialProvider,
	)
}

func templateDynamicCredentialProviderConfig(
	credentials providerInput,
) (*cabpkv1.File, error) {
	return templateCredentialProviderConfig(
		credentials,
		dynamicCredentialProviderConfigPatch,
		kubeletDynamicCredentialProviderConfigOnRemote,
		dynamicCredentialProvider,
	)
}

func templateCredentialProviderConfig(
	credentials providerInput,
	inputTemplate []byte,
	filePath string,
	providerFunc func(
		hasStaticCredentials bool,
		host string,
	) (providerBinary string, providerArgs []string, providerAPIVersion string, err error),
) (*cabpkv1.File, error) {
	mirrorURL, err := url.ParseRequestURI(credentials.URL)
	if err != nil {
		return nil, fmt.Errorf("failed parsing registry mirror: %w", err)
	}

	t := template.New("")
	t.Funcs(map[string]any{
		"urlsMatchStr": credentialprovider.URLsMatchStr,
	})
	t, err = t.Parse(string(inputTemplate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse go template: %w", err)
	}

	mirrorHostWithPath := mirrorURL.Host
	if mirrorURL.Path != "" {
		mirrorHostWithPath = path.Join(mirrorURL.Host, mirrorURL.Path)
	}

	providerBinary, providerArgs, providerAPIVersion, err := providerFunc(
		!credentials.isCredentialsEmpty(),
		mirrorHostWithPath,
	)
	if err != nil {
		return nil, err
	}
	if providerBinary == "" {
		return nil, nil
	}

	templateInput := struct {
		MirrorHost         string
		ProviderBinary     string
		ProviderArgs       []string
		ProviderAPIVersion string
	}{
		MirrorHost:         mirrorHostWithPath,
		ProviderBinary:     providerBinary,
		ProviderArgs:       providerArgs,
		ProviderAPIVersion: providerAPIVersion,
	}

	var b bytes.Buffer
	err = t.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("failed executing template: %w", err)
	}

	return &cabpkv1.File{
		Path:        filePath,
		Content:     b.String(),
		Permissions: "0600",
	}, nil
}

func kubeletCredentialProvider(hasStaticCredentials bool, host string) (
	providerBinary string, providerArgs []string, providerAPIVersion string, err error,
) {
	if needs, err := needCredentialProvider(hasStaticCredentials, host); !needs || err != nil {
		return "", nil, "", err
	}
	return "dynamic-credential-provider",
		[]string{"get-credentials", "-c", kubeletDynamicCredentialProviderConfigOnRemote},
		credentialproviderv1beta1.SchemeGroupVersion.String(),
		nil
}

func dynamicCredentialProvider(hasStaticCredentials bool, host string) (
	providerBinary string, providerArgs []string, providerAPIVersion string, err error,
) {
	if hasStaticCredentials {
		return "static-credential-provider",
			[]string{kubeletStaticCredentialProviderCredentialsOnRemote},
			credentialproviderv1beta1.SchemeGroupVersion.String(),
			nil
	}

	if matches, err := credentialprovider.URLMatchesECR(host); matches || err != nil {
		return "ecr-credential-provider", []string{"get-credentials"},
			credentialproviderv1alpha1.SchemeGroupVersion.String(), err
	}

	if matches, err := credentialprovider.URLMatchesGCR(host); matches || err != nil {
		return "gcr-credential-provider", []string{"get-credentials"},
			credentialproviderv1alpha1.SchemeGroupVersion.String(), err
	}

	if matches, err := credentialprovider.URLMatchesACR(host); matches || err != nil {
		return "acr-credential-provider", []string{
			azureCloudConfigFilePath,
		}, credentialproviderv1alpha1.SchemeGroupVersion.String(), err
	}

	return "", nil, "", nil
}

func needCredentialProvider(hasStaticCredentials bool, host string) (bool, error) {
	if hasStaticCredentials {
		return true, nil
	}
	if matches, err := credentialprovider.URLMatchesECR(host); matches || err != nil {
		//nolint:wrapcheck // No need to wrap this error, it has all context needed.
		return matches, err
	}
	if matches, err := credentialprovider.URLMatchesGCR(host); matches || err != nil {
		//nolint:wrapcheck // No need to wrap this error, it has all context needed.
		return matches, err
	}
	if matches, err := credentialprovider.URLMatchesACR(host); matches || err != nil {
		//nolint:wrapcheck // No need to wrap this error, it has all context needed.
		return matches, err
	}

	return false, nil
}
