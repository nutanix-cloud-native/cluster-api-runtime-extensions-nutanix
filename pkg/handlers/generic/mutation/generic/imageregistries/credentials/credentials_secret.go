// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"path"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

const (
	secretKeyForStaticCredentialProviderConfig = "static-credential-provider" //nolint:gosec // Not a credential.

	dockerHost          = "docker.io"
	registry1DockerHost = "registry-1.docker.io"
	nutanixOrgPath      = "/nutanix"
)

var (
	//go:embed templates/static-credential-provider.json.gotmpl
	staticCredentialProviderConfigPatch []byte

	staticCredentialProviderConfigPatchTemplate = template.Must(
		template.New("").Parse(string(staticCredentialProviderConfigPatch)),
	)
)

func generateCredentialsSecretFile(configs []providerConfig, clusterName string) *bootstrapv1.File {
	if !configsRequireStaticCredentials(configs) {
		return nil
	}
	return &bootstrapv1.File{
		Path: kubeletStaticCredentialProviderCredentialsOnRemote,
		ContentFrom: bootstrapv1.FileSource{
			Secret: bootstrapv1.SecretFileSource{
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
		Password     string //nolint:gosec // Does not contain hard coded credentials.
	}

	var inputs []templateInput
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

		// To maintain existing behavior, include include the path for Nutanix's docker.io org.
		var pathPrefix string
		if isDockerHubNutanixOrgURL(registryURL) {
			pathPrefix = registryURL.Path
		}

		inputs = append(inputs, templateInput{
			RegistryHost: path.Join(registryURL.Host, pathPrefix),
			Username:     config.Username,
			Password:     config.Password,
		})

		// Preserve special handling of "registry-1.docker.io" and add "docker.io"
		// as an alias, carrying the same org/path prefix.
		if registryURL.Host == registry1DockerHost {
			inputs = append(inputs, templateInput{
				RegistryHost: path.Join(dockerHost, pathPrefix),
				Username:     config.Username,
				Password:     config.Password,
			})
		}
	}

	if len(inputs) == 0 {
		return "", nil
	}

	var b bytes.Buffer
	err := staticCredentialProviderConfigPatchTemplate.Execute(&b, inputs)
	if err != nil {
		return "", fmt.Errorf("failed executing template: %w", err)
	}

	return strings.TrimSpace(b.String()), nil
}

func isDockerHubNutanixOrgURL(registryURL *url.URL) bool {
	if registryURL == nil {
		return false
	}

	matchesHost := registryURL.Host == dockerHost || registryURL.Host == registry1DockerHost
	matchesPath := path.Clean(registryURL.Path) == nutanixOrgPath
	return matchesHost && matchesPath
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
