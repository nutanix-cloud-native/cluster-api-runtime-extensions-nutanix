// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// ErrFailedToReleaseReservations means Prism Central still holds addresses for
// this cluster. The caller retries so deleting the cluster does not leak them.
var ErrFailedToReleaseReservations = errors.New(
	"failed to release reserved IP addresses on Prism Central",
)

// errNothingToRelease means the subnet cannot hold a reservation of ours: it no
// longer exists, or it is not IPAM-managed and so was never reservable. Both
// mean there is nothing to release, and treating either as a failure would
// block deleting the cluster forever on a subnet that is already gone.
var errNothingToRelease = errors.New("subnet holds no reservations")

// reservationReleaser is the Prism Central surface the sweep needs. It is an
// interface so the sweep is testable without a Prism Central.
type reservationReleaser interface {
	// ResolveSubnet returns the extID of the subnet named by subnetExtIDOrName.
	ResolveSubnet(ctx context.Context, subnetExtIDOrName, cluster string) (string, error)

	// ReservedIPs returns every reservation on the subnet with the context its
	// reserver stored. The sweep needs all of them because it matches a prefix,
	// and there is no server-side operation that does.
	ReservedIPs(ctx context.Context, subnetExtID string) ([]ReservedIP, error)

	// UnreserveContext releases every address reserved under exactly
	// clientContext, and reports success when there is nothing left to release.
	UnreserveContext(ctx context.Context, subnetExtID, clientContext string) error
}

// ReservedIP is one reservation on a subnet.
type ReservedIP struct {
	Address string
	// ClientContext is whatever the reserver stored. An address reserved by
	// anything other than NKP carries a context this sweep never matches.
	ClientContext string
}

// clusterReservationPrefix is the prefix every address reserved for cluster
// carries, matching the grammar the CCM and the IPAM provider both write:
//
//	nkp:<b64u(clusterUID)>:<b64u(serviceUID)>  a LoadBalancer Service address
//	nkp:<b64u(clusterUID)>:cp                  the API server VIP
//
// Prism caps the context at 64 characters, which is why each UUID is packed
// into 22 characters rather than written out.
func clusterReservationPrefix(clusterUID string) string {
	return "nkp:" + encodeUID(clusterUID) + ":"
}

// encodeUID packs a UID into 22 characters, the same way the CCM does. A
// Kubernetes UID is a UUID, whose 16 raw bytes encode losslessly; anything else
// is hashed to 16 bytes so both sides still agree.
func encodeUID(uid string) string {
	if parsed, err := uuid.Parse(uid); err == nil {
		return base64.RawURLEncoding.EncodeToString(parsed[:])
	}
	sum := sha256.Sum256([]byte(uid))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// releaseClusterReservations releases every address Prism Central still holds
// for cluster on the given subnet.
//
// This is what makes deletion correct in the cases the Service sweep skips by
// design: an unreachable control plane, a cluster that never finished
// provisioning, one already deleting, or one opted out with the GC annotation.
// "The control plane is broken" is the most common reason anyone deletes a
// cluster, and then no Service can be deleted, so the CCM is never asked to
// release anything.
//
// It releases one context at a time rather than one list of addresses.
// Measured on PC 7.6 GA: an IP_ADDRESS_LIST unreserve validates the stored
// context as well as the address, and fails the whole task with
// "not reserved or reserved with a different cookie" when they disagree. So a
// prefix match cannot become a single call, and a CONTEXT unreserve, which
// releases everything under one context, is the operation that fits.
//
// It is idempotent: a second run finds nothing and succeeds.
func releaseClusterReservations(
	ctx context.Context,
	releaser reservationReleaser,
	cluster *clusterv1.Cluster,
	subnet, prismElementCluster string,
	log logr.Logger,
) error {
	clusterUID := string(cluster.GetUID())
	if clusterUID == "" {
		// Without the cluster UID there is no way to tell this cluster's
		// reservations from another's, and releasing the wrong one would strand a
		// live Service. Leaving them is the safe failure.
		return fmt.Errorf("%w: cluster %s/%s has no UID",
			ErrFailedToReleaseReservations, cluster.Namespace, cluster.Name)
	}

	subnetExtID, err := releaser.ResolveSubnet(ctx, subnet, prismElementCluster)
	switch {
	case errors.Is(err, errNothingToRelease):
		log.Info("Subnet no longer exists; nothing to release", "subnet", subnet)
		return nil
	case err != nil:
		// An ambiguous name is still an error. Retrying will not resolve it, but
		// silently skipping could leak, so it is surfaced; an operator who needs
		// the cluster gone can opt out with the GC annotation.
		return fmt.Errorf("%w: resolve subnet %q: %w",
			ErrFailedToReleaseReservations, subnet, err)
	}

	reserved, err := releaser.ReservedIPs(ctx, subnetExtID)
	switch {
	case errors.Is(err, errNothingToRelease):
		log.Info("Subnet is not IPAM-managed or is gone; nothing to release", "subnet", subnetExtID)
		return nil
	case err != nil:
		return fmt.Errorf("%w: list reservations on subnet %s: %w",
			ErrFailedToReleaseReservations, subnetExtID, err)
	}

	stale := reservationContextsForCluster(reserved, clusterUID)
	if len(stale) == 0 {
		return nil
	}

	log.Info("Releasing reserved IP addresses on Prism Central",
		"subnet", subnetExtID, "reservations", len(stale))

	// Keep going after a failure so one unreleasable reservation does not hide
	// the rest, and report every context that is still held.
	var failed []string
	for _, clientContext := range stale {
		if err := releaser.UnreserveContext(ctx, subnetExtID, clientContext); err != nil {
			log.Error(err, "Failed to release a reservation", "clientContext", clientContext)
			failed = append(failed, clientContext)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%w: subnet %s still holds %s",
			ErrFailedToReleaseReservations, subnetExtID, strings.Join(failed, ", "))
	}
	return nil
}

// reservationContextsForCluster returns the distinct reservation contexts that
// belong to clusterUID, sorted so retries issue the same calls in the same
// order. A reservation made by anything other than NKP, or by another cluster,
// is never returned: the subnet is shared with VMs and may be shared with other
// clusters.
func reservationContextsForCluster(reserved []ReservedIP, clusterUID string) []string {
	prefix := clusterReservationPrefix(clusterUID)
	matched := make([]string, 0, len(reserved))
	for _, r := range reserved {
		if !strings.HasPrefix(r.ClientContext, prefix) {
			continue
		}
		matched = append(matched, r.ClientContext)
	}
	slices.Sort(matched)
	return slices.Compact(matched)
}
