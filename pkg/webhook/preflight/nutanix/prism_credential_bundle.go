// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// ErrPrismCredentialsSecretNotFound indicates the referenced credentials Secret does not exist.
var ErrPrismCredentialsSecretNotFound = errors.New("prism credentials secret not found")

// LoadPrismCredentialBundle resolves Prism Central endpoint credentials from the cluster configuration
// and the referenced Secret (same inputs as the NutanixCredentials preflight check).
func LoadPrismCredentialBundle(
	ctx context.Context,
	kclient ctrlclient.Client,
	namespace string,
	nutanixClusterConfigSpec *carenv1.NutanixClusterConfigSpec,
) (prismgoclient.Credentials, string, error) {
	if nutanixClusterConfigSpec == nil || nutanixClusterConfigSpec.Nutanix == nil {
		return prismgoclient.Credentials{}, "", fmt.Errorf("nutanix cluster configuration is missing")
	}

	prismCentralEndpointSpec := nutanixClusterConfigSpec.Nutanix.PrismCentralEndpoint

	host, port, err := prismCentralEndpointSpec.ParseURL()
	if err != nil {
		return prismgoclient.Credentials{}, "", err
	}

	credentialsSecret := &corev1.Secret{}
	err = kclient.Get(
		ctx,
		types.NamespacedName{
			Namespace: namespace,
			Name:      prismCentralEndpointSpec.Credentials.SecretRef.Name,
		},
		credentialsSecret,
	)
	if apierrors.IsNotFound(err) {
		return prismgoclient.Credentials{}, "", fmt.Errorf(
			"%w: %s",
			ErrPrismCredentialsSecretNotFound,
			prismCentralEndpointSpec.Credentials.SecretRef.Name,
		)
	}
	if err != nil {
		return prismgoclient.Credentials{}, "", fmt.Errorf(
			"prism secret get for %q: %w",
			prismCentralEndpointSpec.Credentials.SecretRef.Name,
			err,
		)
	}

	if len(credentialsSecret.Data) == 0 {
		return prismgoclient.Credentials{}, "", fmt.Errorf(
			"credentials Secret %q is empty",
			prismCentralEndpointSpec.Credentials.SecretRef.Name,
		)
	}

	data, ok := credentialsSecret.Data[credentialsSecretDataKey]
	if !ok {
		return prismgoclient.Credentials{}, "", fmt.Errorf(
			"credentials Secret %q does not contain key %q",
			prismCentralEndpointSpec.Credentials.SecretRef.Name,
			credentialsSecretDataKey,
		)
	}

	parsedCredentials, err := prismcredentials.ParseCredentials(data)
	if err != nil {
		return prismgoclient.Credentials{}, "", fmt.Errorf("failed to parse Prism Central credentials: %w", err)
	}

	var additionalTrustBundlePEM string
	if prismCentralEndpointSpec.AdditionalTrustBundle != "" {
		decoded, err := base64.StdEncoding.DecodeString(prismCentralEndpointSpec.AdditionalTrustBundle)
		if err != nil {
			return prismgoclient.Credentials{}, "", fmt.Errorf(
				"failed to decode Prism Central additionalTrustBundle (expected base64 PEM): %w",
				err,
			)
		}
		additionalTrustBundlePEM = string(decoded)
	}

	credentials := prismgoclient.Credentials{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		URL:      fmt.Sprintf("https://%s:%d", host, port),
		Username: parsedCredentials.Username,
		Password: parsedCredentials.Password,
		Insecure: prismCentralEndpointSpec.Insecure,
		APIKey:   parsedCredentials.APIKey,
	}

	return credentials, additionalTrustBundlePEM, nil
}
