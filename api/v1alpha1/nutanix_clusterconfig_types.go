// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"net/url"
	"strconv"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
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

	// failureDomains configures failure domains information for the Nutanix platform.
	// When set, the failure domains defined here may be used to spread Machines across
	// prism element clusters to improve fault tolerance of the cluster.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:Optional
	FailureDomains []NutanixFailureDomain `json:"failureDomains,omitempty"`
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
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=`byte`
	AdditionalTrustBundle string `json:"additionalTrustBundle,omitempty"`

	// A reference to the Secret for credential information for the target Prism Central instance.
	// +kubebuilder:validation:Required
	Credentials NutanixPrismCentralEndpointCredentials `json:"credentials"`
}

type NutanixPrismCentralEndpointCredentials struct {
	// A reference to the Secret containing the Prism Central credentials.
	// +kubebuilder:validation:Required
	SecretRef LocalObjectReference `json:"secretRef"`
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

// NutanixFailureDomains is a list of FDs.
type NutanixFailureDomains []NutanixFailureDomain

// NutanixFailureDomain configures failure domain information for Nutanix.
type NutanixFailureDomain struct {
	// name defines the unique name of a failure domain.
	// Name is required and must be at most 64 characters in length.
	// It must consist of only lower case alphanumeric characters and hyphens (-).
	// It must start and end with an alphanumeric character.
	// This value is arbitrary and is used to identify the failure domain within the platform.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`[a-z0-9]([-a-z0-9]*[a-z0-9])?`
	Name string `json:"name"`

	// cluster is to identify the cluster (the Prism Element under management of the Prism Central),
	// in which the Machine's VM will be created. The cluster identifier (uuid or name) can be obtained
	// from the Prism Central console or using the prism_central API.
	// +kubebuilder:validation:Required
	Cluster capxv1.NutanixResourceIdentifier `json:"cluster"`

	// subnets holds a list of identifiers (one or more) of the cluster's network subnets
	// for the Machine's VM to connect to. The subnet identifiers (uuid or name) can be
	// obtained from the Prism Central console or using the prism_central API.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Subnets []capxv1.NutanixResourceIdentifier `json:"subnets"`

	// indicates if a failure domain is suited for control plane nodes
	// +kubebuilder:validation:Required
	ControlPlane bool `json:"controlPlane,omitempty"`
}
