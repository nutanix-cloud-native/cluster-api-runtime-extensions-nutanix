// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package nutanix provides shared Prism Central access helpers.
package nutanix

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	prismtypes "github.com/nutanix-cloud-native/prism-go-client/environment/types"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const prismCredentialsSecretDataKey = "credentials"

//nolint:gochecknoglobals // Prism clients are shared to preserve connection pooling across callers.
var prismCentralClientCache = v4.NewClientCache()

// VersionForCluster resolves the raw Prism Central version configured for a Cluster.
// It deliberately does not inspect preflight-skip annotations: callers use the version
// for persisted policy decisions rather than validation.
func PrismCentralVersionForCluster(
	ctx context.Context,
	kclient ctrlclient.Client,
	cluster *clusterv1.Cluster,
	spec *carenv1.NutanixClusterConfigSpec,
) (string, error) {
	if cluster == nil || spec == nil || spec.Nutanix == nil {
		return "", nil
	}

	credentials, trustBundle, err := loadCredentials(ctx, kclient, cluster.Namespace, spec)
	if err != nil {
		return "", err
	}
	endpoint, err := managementEndpoint(&credentials, trustBundle)
	if err != nil {
		return "", err
	}
	client, err := prismCentralClientCache.GetOrCreate(&cacheParams{
		clusterNamespacedName: types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace},
		endpoint:              endpoint,
	})
	if err != nil {
		return "", fmt.Errorf("create Prism Central client: %w", err)
	}
	version, err := client.DomainManager.GetPrismCentralVersion(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(version), nil
}

func loadCredentials(
	ctx context.Context,
	kclient ctrlclient.Client,
	namespace string,
	spec *carenv1.NutanixClusterConfigSpec,
) (prismgoclient.Credentials, string, error) {
	endpoint := spec.Nutanix.PrismCentralEndpoint
	host, port, err := endpoint.ParseURL()
	if err != nil {
		return prismgoclient.Credentials{}, "", err
	}
	secret := &corev1.Secret{}
	if err := kclient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      endpoint.Credentials.SecretRef.Name,
	}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return prismgoclient.Credentials{}, "", fmt.Errorf(
				"prism credentials secret %q not found", endpoint.Credentials.SecretRef.Name,
			)
		}
		return prismgoclient.Credentials{}, "", err
	}
	data, ok := secret.Data[prismCredentialsSecretDataKey]
	if !ok {
		return prismgoclient.Credentials{}, "", fmt.Errorf(
			"credentials Secret %q does not contain key %q",
			endpoint.Credentials.SecretRef.Name, prismCredentialsSecretDataKey,
		)
	}
	parsed, err := prismcredentials.ParseCredentials(data)
	if err != nil {
		return prismgoclient.Credentials{}, "", fmt.Errorf("parse Prism Central credentials: %w", err)
	}
	trustBundle := ""
	if endpoint.AdditionalTrustBundle != "" {
		decoded, err := base64.StdEncoding.DecodeString(endpoint.AdditionalTrustBundle)
		if err != nil {
			return prismgoclient.Credentials{}, "", fmt.Errorf("decode Prism Central trust bundle: %w", err)
		}
		trustBundle = string(decoded)
	}
	return prismgoclient.Credentials{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		URL:      fmt.Sprintf("https://%s:%d", host, port),
		Username: parsed.Username,
		Password: parsed.Password,
		APIKey:   parsed.APIKey,
		Insecure: endpoint.Insecure,
	}, trustBundle, nil
}

func managementEndpoint(
	credentials *prismgoclient.Credentials,
	trustBundle string,
) (*prismtypes.ManagementEndpoint, error) {
	urlString := credentials.URL
	if !strings.HasPrefix(urlString, "http://") && !strings.HasPrefix(urlString, "https://") {
		urlString = "https://" + urlString
	}
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return nil, fmt.Errorf("parse Prism Central URL %q: %w", credentials.URL, err)
	}
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("prism central URL %q has no host", credentials.URL)
	}
	return &prismtypes.ManagementEndpoint{
		Address:               parsedURL,
		Insecure:              credentials.Insecure,
		AdditionalTrustBundle: trustBundle,
		ApiCredentials: prismtypes.ApiCredentials{
			Username: credentials.Username,
			Password: credentials.Password,
			APIKey:   credentials.APIKey,
		},
	}, nil
}

type cacheParams struct {
	clusterNamespacedName types.NamespacedName
	endpoint              *prismtypes.ManagementEndpoint
}

func (p *cacheParams) ManagementEndpoint() prismtypes.ManagementEndpoint {
	return *p.endpoint
}

func (p *cacheParams) Key() string {
	return p.clusterNamespacedName.String()
}
