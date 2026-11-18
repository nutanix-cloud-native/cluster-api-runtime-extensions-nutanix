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
// when the cluster reserves one.
//
// A host and a reservation are mutually exclusive - the cluster webhook refuses
// a cluster that sets both - so in practice the host is always empty here. The
// guards below are defence in depth: whatever a cluster was admitted with, an
// address an operator named is never silently replaced.
//
// Reports whether an address was injected. It is false both when the cluster
// does not reserve and when the reservation has not been allocated yet; the
// caller does not need to tell those apart, because BeforeClusterCreate is what
// holds creation until an address exists.
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
