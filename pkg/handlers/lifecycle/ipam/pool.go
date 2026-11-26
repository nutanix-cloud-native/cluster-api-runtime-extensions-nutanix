// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/ipamendpoint"
)

// NutanixIPPool is the pool kind the Nutanix IPAM provider reconciles. It is
// built unstructured rather than from a vendored type: the only fields needed
// are the Prism Central connection, which this cluster already carries, and the
// subnet. Vendoring the provider's API would add a module dependency and an
// api.sync entry to set three strings.
var nutanixIPPoolGVK = schema.GroupVersionKind{
	Group:   "ipam.cluster.x-k8s.io",
	Version: "v1alpha1",
	Kind:    "NutanixIPPool",
}

// poolName is the pool and claim name for a cluster's API server address. It is
// derived from the cluster name so it is stable across retries, and both
// objects carry an owner reference to the Cluster, so deleting the cluster
// garbage-collects them and the provider then releases the address.
func poolName(clusterName string) string {
	return ipamendpoint.ClaimName(clusterName)
}

// newNutanixIPPool builds the pool for a cluster's API server address from the
// cluster's own Prism Central configuration, so the pool can never point at a
// different Prism Central than the cluster itself.
func newNutanixIPPool(
	name, namespace string,
	endpoint *carenv1.NutanixPrismCentralEndpointSpec,
	reservation *carenv1.ControlPlaneIPReservation,
) (*unstructured.Unstructured, error) {
	if endpoint == nil {
		return nil, fmt.Errorf("nutanix.prismCentralEndpoint is not set")
	}
	if reservation == nil || reservation.Subnet == "" {
		return nil, fmt.Errorf("controlPlaneEndpoint.virtualIP.configuration.ipReservation.subnet is not set")
	}
	host, port, err := endpoint.ParseURL()
	if err != nil {
		return nil, fmt.Errorf("parse nutanix.prismCentralEndpoint url: %w", err)
	}
	if endpoint.Credentials.SecretRef.Name == "" {
		return nil, fmt.Errorf("nutanix.prismCentralEndpoint.credentials.secretRef.name is not set")
	}

	prismCentral := map[string]any{
		"address":  host,
		"port":     int64(port),
		"insecure": endpoint.Insecure,
		"credentialsSecretRef": map[string]any{
			"name": endpoint.Credentials.SecretRef.Name,
		},
	}
	if endpoint.AdditionalTrustBundle != "" {
		// The provider's field is []byte, which is base64 in JSON, and this value
		// is already base64, so it is passed through rather than decoded.
		prismCentral["additionalTrustBundle"] = map[string]any{
			"trustBundleData": endpoint.AdditionalTrustBundle,
		}
	}

	spec := map[string]any{
		"prismCentral": prismCentral,
		"subnet":       reservation.Subnet,
	}
	if reservation.Cluster != nil && *reservation.Cluster != "" {
		spec["cluster"] = *reservation.Cluster
	}

	pool := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	pool.SetGroupVersionKind(nutanixIPPoolGVK)
	pool.SetName(name)
	pool.SetNamespace(namespace)
	return pool, nil
}
