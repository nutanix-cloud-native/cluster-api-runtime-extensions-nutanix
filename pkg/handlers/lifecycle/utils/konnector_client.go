// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package utils provides utility functions for lifecycle handlers.
package utils

import (
	"context"
	"encoding/base64"
	"fmt"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	konnectorprismgoclient "github.com/nutanix-cloud-native/prism-go-client/karbon"
)

// NewPrismCentralKonnectorClient creates a new Prism Konnector client that is used to call the Konnector APIs.
func NewPrismCentralKonnectorClient(credentials *prismgoclient.Credentials, additionalTrustBundle string,
	clientOpts ...konnectorprismgoclient.ClientOption,
) (*PrismCentralKonnectorClient, error) {
	if credentials == nil {
		return nil, fmt.Errorf(
			" Prism Central credentials cannot be nil, needed to create Prism Central Konnector client",
		)
	}

	if additionalTrustBundle != "" {
		certBytes, err := base64.StdEncoding.DecodeString(additionalTrustBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 certificate: %w", err)
		}
		clientOpts = append(clientOpts, konnectorprismgoclient.WithPEMEncodedCertBundle(certBytes))
		// Set insecure to false if trust bundle is provided.
		credentials.Insecure = false
	}

	prismCentralKonnectorClient, err := konnectorprismgoclient.NewKarbonAPIClient(*credentials, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create prism konnector client: %w", err)
	}

	return &PrismCentralKonnectorClient{prismCentralKonnectorClient: prismCentralKonnectorClient}, nil
}

// PrismCentralKonnectorClient wraps the Prism Central Konnector client.
type PrismCentralKonnectorClient struct {
	prismCentralKonnectorClient *konnectorprismgoclient.Client
}

// GetClusterRegistration retrieves the cluster registration from Prism Central.
func (pc *PrismCentralKonnectorClient) GetClusterRegistration(
	ctx context.Context,
	clusterUUID string,
) (*konnectorprismgoclient.K8sClusterRegistration, error) {
	if pc == nil || pc.prismCentralKonnectorClient == nil {
		return nil, fmt.Errorf("could not connect to API server on PC: client is nil")
	}
	clusterRegistration, err := pc.prismCentralKonnectorClient.ClusterRegistrationOperations.GetK8sRegistration(
		ctx,
		clusterUUID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes cluster(%s) registration: %w", clusterUUID, err)
	}
	return clusterRegistration, nil
}
