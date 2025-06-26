// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

type RegistryMetadata struct {
	HelmReleaseName      string
	HelmReleaseNamespace string

	Replicas int32

	Namespace           string
	ServiceName         string
	HeadlessServiceName string
	ServiceIP           string
	ServicePort         int32
	HeadlessServicePort int32

	// AddressFromClusterNetwork is the FQDN of the registry service as seen from the cluster network.
	AddressFromClusterNetwork string

	TLSSecretName string
	// CASecretName is the name of the Secret on the management cluster that contains the CA certificate.
	CASecretName           string
	CertificateDNSNames    []string
	CertificateIPAddresses []string
}

// GetRegistryMetadata returns the registry metadata for a given cluster based on the addon provider.
func GetRegistryMetadata(cluster *clusterv1.Cluster) (*RegistryMetadata, error) {
	// Only a single registry provider is supported for now
	return getRegistryMetadataForCNCFDistribution(cluster)
}

func getRegistryMetadataForCNCFDistribution(cluster *clusterv1.Cluster) (*RegistryMetadata, error) {
	const (
		defaultHelmReleaseName      = "cncf-distribution-registry"
		defaultHelmReleaseNamespace = "registry-system"

		replicas = 2

		workloadName        = "cncf-distribution-registry-docker-registry"
		serviceName         = "cncf-distribution-registry-docker-registry"
		headlessServiceName = "cncf-distribution-registry-docker-registry-headless"
		servicePort         = 443
		// This needs to match Pod's container port
		headlessServicePort = 5000

		tlsSecretName = "registry-tls"
	)
	serviceIP, err := ServiceIPForCluster(cluster)
	if err != nil {
		return nil, fmt.Errorf("error getting service IP for the CNCF distribution registry: %w", err)
	}
	addressFromClusterNetwork := fmt.Sprintf(
		"%s.%s.svc.cluster.local:%d",
		serviceName,
		defaultHelmReleaseNamespace,
		servicePort,
	)
	certificateDNSNames := getCertificateDNSNames(
		workloadName,
		headlessServiceName,
		defaultHelmReleaseNamespace,
		replicas,
	)
	certificateIPAddresses := getCertificateIPAddresses(serviceIP)

	return &RegistryMetadata{
		HelmReleaseName:      defaultHelmReleaseName,
		HelmReleaseNamespace: defaultHelmReleaseNamespace,

		Replicas: replicas,

		Namespace:           defaultHelmReleaseNamespace,
		ServiceName:         serviceName,
		HeadlessServiceName: headlessServiceName,
		ServiceIP:           serviceIP,
		ServicePort:         servicePort,
		HeadlessServicePort: headlessServicePort,

		AddressFromClusterNetwork: addressFromClusterNetwork,

		TLSSecretName:          tlsSecretName,
		CASecretName:           handlersutils.SecretNameForRegistryAddonCA(cluster),
		CertificateDNSNames:    certificateDNSNames,
		CertificateIPAddresses: certificateIPAddresses,
	}, nil
}

func getCertificateDNSNames(workloadName, headlessServiceName, namespace string, replicas int) []string {
	names := []string{
		workloadName,
		fmt.Sprintf("%s.%s", workloadName, namespace),
		fmt.Sprintf("%s.%s.svc", workloadName, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", workloadName, namespace),
	}
	for i := 0; i < replicas; i++ {
		names = append(names,
			[]string{
				fmt.Sprintf("%s-%d", workloadName, i),
				fmt.Sprintf("%s-%d.%s.%s", workloadName, i, headlessServiceName, namespace),
				fmt.Sprintf("%s-%d.%s.%s.svc", workloadName, i, headlessServiceName, namespace),
				fmt.Sprintf(
					"%s-%d.%s.%s.svc.cluster.local",
					workloadName, i, headlessServiceName, namespace,
				),
			}...,
		)
	}

	return names
}

func getCertificateIPAddresses(serviceIP string) []string {
	return []string{
		serviceIP,
		"127.0.0.1",
	}
}
