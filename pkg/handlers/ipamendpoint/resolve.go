// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ipamendpoint resolves the API server address a cluster reserved from
// an IPAM provider.
//
// The address is deliberately NOT written back into the Cluster. NKP owns the
// CAPI Cluster's topology through its own NKPCluster resource and its webhook
// rejects any direct modification of it:
//
//	spec.topology: Forbidden: CAPI Cluster topology spec is managed by
//	NKPCluster ... and must not be modified directly.
//
// So the reservation lives only in the IPAddress the provider allocated, and
// the mutation handlers read it from there on every topology reconcile. That is
// also the CAPI-native shape: a generated template is a pure function of the
// cluster plus whatever the handlers look up, and it is recomputed every time
// rather than persisted once.
package ipamendpoint

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// ClaimName is the name of the pool and the claim holding a cluster's API
// server address. It is derived from the cluster name so every component
// computes the same name without having to store it anywhere.
func ClaimName(clusterName string) string {
	return clusterName + "-control-plane-endpoint"
}

// Resolve fills in the API server address on spec from the reserved address
// when the cluster reserves one and no host was given.
//
// An explicitly configured host always wins: an operator who named a host is
// pointing at something that already exists, and the reservation is then only
// the virtual IP. Returns false when there is nothing to resolve, so callers
// can tell "not reserved" from "reserved but not allocated yet".
func Resolve(
	ctx context.Context,
	c ctrlclient.Client,
	cluster ctrlclient.ObjectKey,
	spec *v1alpha1.ControlPlaneEndpointSpec,
) (bool, error) {
	if spec == nil || !reserves(spec) {
		return false, nil
	}

	address := &ipamv1.IPAddress{}
	key := ctrlclient.ObjectKey{Namespace: cluster.Namespace, Name: ClaimName(cluster.Name)}
	if err := c.Get(ctx, key, address); err != nil {
		// Not allocated yet, or the provider's types are not installed. Either
		// way there is no address to inject; BeforeClusterCreate is what holds
		// cluster creation until there is one, so this is not fatal here.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, fmt.Errorf("get IPAddress %s: %w", key, err)
	}
	if address.Spec.Address == "" {
		return false, nil
	}

	if spec.Host == "" {
		spec.Host = address.Spec.Address
	}
	if spec.VirtualIPSpec != nil &&
		spec.VirtualIPSpec.Configuration != nil &&
		spec.VirtualIPSpec.Configuration.Address == "" {
		spec.VirtualIPSpec.Configuration.Address = address.Spec.Address
	}
	return true, nil
}

// reserves reports whether the cluster asked for its API server address to be
// reserved rather than naming one.
func reserves(spec *v1alpha1.ControlPlaneEndpointSpec) bool {
	return spec.VirtualIPSpec != nil &&
		spec.VirtualIPSpec.Configuration != nil &&
		spec.VirtualIPSpec.Configuration.IPReservation != nil
}
