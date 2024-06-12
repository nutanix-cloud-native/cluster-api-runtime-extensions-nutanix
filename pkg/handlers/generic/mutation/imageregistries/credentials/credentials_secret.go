// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

const (
	secretKeyForStaticCredentialProviderConfig = "static-credential-provider" //nolint:gosec // Not a credential.
)

var (
	//go:embed templates/static-credential-provider.json.gotmpl
	staticCredentialProviderConfigPatch []byte

	staticCredentialProviderConfigPatchTemplate = template.Must(
		template.New("").Parse(string(staticCredentialProviderConfigPatch)),
	)
)

func generateCredentialsSecretFile(configs []providerConfig, clusterName string) *cabpkv1.File {
	if !configsRequireStaticCredentials(configs) {
		return nil
	}
	return &cabpkv1.File{
		Path: kubeletStaticCredentialProviderCredentialsOnRemote,
		ContentFrom: &cabpkv1.FileSource{
			Secret: cabpkv1.SecretFileSource{
				Name: credentialSecretName(clusterName),
				Key:  secretKeyForStaticCredentialProviderConfig,
			},
		},
		Permissions: "0600",
	}
}

// generateCredentialsSecret generates a Secret containing the config for the image registry.
// The function needs the cluster name to add the required move and cluster name labels.
func generateCredentialsSecret(
	configs []providerConfig, clusterName, namespace string,
) (*corev1.Secret, error) {
	if !configsRequireStaticCredentials(configs) {
		return nil, nil
	}

	staticCredentialProviderSecretContents, err := kubeletStaticCredentialProviderSecretContents(
		configs,
	)
	if err != nil {
		return nil, err
	}
	secretData := map[string]string{
		secretKeyForStaticCredentialProviderConfig: staticCredentialProviderSecretContents,
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      credentialSecretName(clusterName),
			Namespace: namespace,
			Labels:    utils.NewLabels(utils.WithMove(), utils.WithClusterName(clusterName)),
		},
		StringData: secretData,
		Type:       corev1.SecretTypeOpaque,
	}, nil
}

func kubeletStaticCredentialProviderSecretContents(configs []providerConfig) (string, error) {
	type templateInput struct {
		RegistryHost string
		Username     string
		Password     string
		Separator    string
	}

	inputs := make([]templateInput, 0)
	for _, config := range configs {
		requiresStaticCredentials, err := config.requiresStaticCredentials()
		if err != nil {
			return "", fmt.Errorf(
				"error determining if Image Registry is a supported provider: %w",
				err,
			)
		}
		if !requiresStaticCredentials {
			continue
		}

		registryURL, err := url.ParseRequestURI(config.URL)
		if err != nil {
			return "", fmt.Errorf("failed parsing registry URL: %w", err)
		}

		separator := ","
		inputs = append(inputs, templateInput{
			RegistryHost: registryURL.Host,
			Username:     config.Username,
			Password:     config.Password,
			Separator:    separator,
		})

		// Preserve special handling of "registry-1.docker.io" and add "docker.io" as an alias.
		if registryURL.Host == "registry-1.docker.io" {
			inputs = append(inputs, templateInput{
				RegistryHost: "docker.io",
				Username:     config.Username,
				Password:     config.Password,
				Separator:    separator,
			})
		}
	}

	if len(inputs) == 0 {
		return "", nil
	}

	// The template is a JSON array, so we need a "," between each entry except the last one.
	inputs[len(inputs)-1].Separator = ""

	var b bytes.Buffer
	err := staticCredentialProviderConfigPatchTemplate.Execute(&b, inputs)
	if err != nil {
		return "", fmt.Errorf("failed executing template: %w", err)
	}

	return strings.TrimSpace(b.String()), nil
}

func configsRequireStaticCredentials(configs []providerConfig) bool {
	for _, config := range configs {
		requiresStaticCredentials, err := config.requiresStaticCredentials()
		if err != nil {
			return false
		}
		if requiresStaticCredentials {
			return true
		}
	}
	return false
}

func credentialSecretName(clusterName string) string {
	return fmt.Sprintf("%s-static-credential-provider-response", clusterName)
}
