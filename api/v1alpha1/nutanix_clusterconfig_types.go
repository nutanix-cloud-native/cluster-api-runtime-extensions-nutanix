// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"net/url"
	"strconv"
)

const (
	DefaultPrismCentralPort = 9440
)

// NutanixSpec defines the desired state of NutanixCluster.
type NutanixSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// host can be either DNS name or ip address
	// +kubebuilder:validation:Required
	ControlPlaneEndpoint ControlPlaneEndpointSpec `json:"controlPlaneEndpoint"`

	// Nutanix Prism Central endpoint configuration.
	// +kubebuilder:validation:Required
	PrismCentralEndpoint NutanixPrismCentralEndpointSpec `json:"prismCentralEndpoint"`
}

type NutanixPrismCentralEndpointSpec struct {
	// The URL of Nutanix Prism Central, can be DNS name or an IP address.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=`uri`
	// +kubebuilder:validation:Pattern=`^https://`
	URL string `json:"url"`

	// use insecure connection to Prism Central endpoint
	// +kubebuilder:validation:Optional
	Insecure bool `json:"insecure"`

	// A base64 PEM encoded x509 cert for the RootCA that was used to create
	// the certificate for a Prism Central that uses certificates that were issued by a non-publicly trusted RootCA.
	// The trust bundle is added to the cert pool used to authenticate the TLS connection to the Prism Central.
	// +kubebuilder:validation:Format=`byte`
	// +kubebuilder:validation:Optional
	AdditionalTrustBundle string `json:"additionalTrustBundle,omitempty"`

	// A reference to the Secret for credential information for the target Prism Central instance.
	// +kubebuilder:validation:Required
	Credentials LocalObjectReference `json:"credentials"`
}

//nolint:gocritic // No need for named return values
func (s NutanixPrismCentralEndpointSpec) ParseURL() (string, int32, error) {
	var prismCentralURL *url.URL
	prismCentralURL, err := url.Parse(s.URL)
	if err != nil {
		return "", -1, fmt.Errorf("error parsing Prism Central URL: %w", err)
	}

	hostname := prismCentralURL.Hostname()

	// return early with the default port if no port is specified
	if prismCentralURL.Port() == "" {
		return hostname, DefaultPrismCentralPort, nil
	}

	port, err := strconv.ParseInt(prismCentralURL.Port(), 10, 32)
	if err != nil {
		return "", -1, fmt.Errorf("error converting port to int: %w", err)
	}

	return hostname, int32(port), nil
}
