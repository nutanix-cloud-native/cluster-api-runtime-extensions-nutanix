// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"strings"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// instanceMetadataIP is the IPv4 address used to retrieve
	// instance metadata in AWS, Azure, OpenStack, etc.
	instanceMetadataIP = "169.254.169.254"
)

// HTTPProxy required for providing proxy configuration.
type HTTPProxy struct {
	// HTTP proxy value.
	HTTP string `json:"http,omitempty"`

	// HTTPS proxy value.
	HTTPS string `json:"https,omitempty"`

	// AdditionalNo Proxy list that will be added to the automatically calculated
	// values that will apply no_proxy configuration for cluster internal network.
	// Default values: localhost,127.0.0.1,<POD_NETWORK>,<SERVICE_NETWORK>,kubernetes
	//   ,kubernetes.default,.svc,.svc.<SERVICE_DOMAIN>
	AdditionalNo []string `json:"additionalNo"`
}

// GenerateNoProxy creates default NO_PROXY values that should be applied on cluster
// in any environment and are preventing the use of proxy for cluster internal
// networking. It appends additional values from HTTPProxy.AdditionalNo.
func (p *HTTPProxy) GenerateNoProxy(cluster *clusterv1.Cluster) []string {
	noProxy := []string{
		"localhost",
		"127.0.0.1",
	}

	if cluster.Spec.ClusterNetwork != nil &&
		cluster.Spec.ClusterNetwork.Pods != nil {
		noProxy = append(noProxy, cluster.Spec.ClusterNetwork.Pods.CIDRBlocks...)
	}

	if cluster.Spec.ClusterNetwork != nil &&
		cluster.Spec.ClusterNetwork.Services != nil {
		noProxy = append(noProxy, cluster.Spec.ClusterNetwork.Services.CIDRBlocks...)
	}

	serviceDomain := "cluster.local"
	if cluster.Spec.ClusterNetwork != nil &&
		cluster.Spec.ClusterNetwork.ServiceDomain != "" {
		serviceDomain = cluster.Spec.ClusterNetwork.ServiceDomain
	}

	noProxy = append(
		noProxy,
		"kubernetes",
		"kubernetes.default",
		".svc",
		// append .svc.<SERVICE_DOMAIN>
		fmt.Sprintf(".svc.%s", strings.TrimLeft(serviceDomain, ".")),
	)

	if cluster.Spec.InfrastructureRef == nil {
		return append(noProxy, p.AdditionalNo...)
	}

	// Add infra-specific entries
	switch cluster.Spec.InfrastructureRef.Kind {
	case "AWSCluster", "AWSManagedCluster":
		noProxy = append(
			noProxy,
			// Exclude the instance metadata service
			instanceMetadataIP,
			// Exclude the control plane endpoint
			".elb.amazonaws.com",
		)
	case "AzureCluster", "AzureManagedControlPlane":
		noProxy = append(
			noProxy,
			// Exclude the instance metadata service
			instanceMetadataIP,
		)
	case "GCPCluster":
		noProxy = append(
			noProxy,
			// Exclude the instance metadata service
			instanceMetadataIP,
			// Exclude aliases for instance metadata service.
			// See https://cloud.google.com/vpc/docs/special-configurations
			"metadata",
			"metadata.google.internal",
		)
	default:
		// Unknown infrastructure. Do nothing.
	}
	return append(noProxy, p.AdditionalNo...)
}
