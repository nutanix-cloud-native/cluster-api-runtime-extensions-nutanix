// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ipam reserves the API server address for a cluster that does not
// name one, by claiming it from a Nutanix subnet through the Cluster API IPAM
// contract. It runs before the cluster is created, because the address has to
// exist before kube-vip is templated with it.
package ipam

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
)

// retryAfterSeconds is how long CAPI waits before asking again while the
// address is still being reserved. The reservation is a Prism task, so seconds
// rather than milliseconds, and the hook must not block a worker waiting.
const retryAfterSeconds = 5

// ErrAddressReservationFailed means the API server address could not be
// reserved. Cluster creation does not proceed: a cluster whose kube-vip has no
// address would come up with an unreachable API server.
var ErrAddressReservationFailed = errors.New("failed to reserve the API server address")

// ErrIPAMProviderNotInstalled means the NutanixIPPool kind is absent, so no
// controller can service a claim. It is terminal: unlike every other failure
// here, retrying will never resolve it, and this hook runs in the bootstrap
// cluster during a self-managed create, where a retry loop hangs the create
// with nothing to point at.
var ErrIPAMProviderNotInstalled = errors.New(
	"the Nutanix IPAM provider is not installed, so the API server address cannot be reserved; " +
		"install it, or set controlPlaneEndpoint.host instead of ipReservation",
)

type ControlPlaneEndpointIPAM struct {
	client ctrlclient.Client
}

var (
	_ handlers.Named                = &ControlPlaneEndpointIPAM{}
	_ lifecycle.BeforeClusterCreate = &ControlPlaneEndpointIPAM{}
)

func New(client ctrlclient.Client) *ControlPlaneEndpointIPAM {
	return &ControlPlaneEndpointIPAM{client: client}
}

func (h *ControlPlaneEndpointIPAM) Name() string {
	return "ControlPlaneEndpointIPAM"
}

// BeforeClusterCreate reserves the API server address when the cluster asks for
// one, and does nothing at all otherwise.
//
// The address must be in the cluster's topology variable before CAPI reconciles
// the topology, because that is what templates kube-vip's static pod and what
// the CNI and the CCM read. So this runs as a lifecycle hook and writes the
// address back into controlPlaneEndpoint.host.
//
// Reserving is asynchronous: the claim is created, and if the provider has not
// answered yet the hook reports a retry rather than waiting. CAPI calls it
// again until the address exists.
func (h *ControlPlaneEndpointIPAM) BeforeClusterCreate(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterCreateRequest,
	resp *runtimehooksv1.BeforeClusterCreateResponse,
) {
	cluster := &req.Cluster
	log := ctrl.LoggerFrom(ctx).WithValues("cluster", ctrlclient.ObjectKeyFromObject(cluster))

	clusterConfig, err := apivariables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("%v: read cluster config: %v", ErrAddressReservationFailed, err))
		return
	}

	reservation, endpoint := reservationSource(clusterConfig)
	if reservation == nil {
		// The operator named the address themselves. Nothing to reserve.
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		return
	}

	address, err := h.reserve(ctx, cluster, endpoint, reservation, log)
	switch {
	case errors.Is(err, errReservationPending):
		log.Info("Waiting for the API server address to be reserved")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		// Must be set on the typed response: the shared registry handler funnels
		// through CommonResponse, which carries only status and message.
		resp.SetRetryAfterSeconds(retryAfterSeconds)
		return
	case errors.Is(err, ErrIPAMProviderNotInstalled):
		// Terminal. No RetryAfterSeconds, so the failure is reported once with a
		// message naming the cause instead of looping.
		log.Error(err, "Cannot reserve the API server address")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	case err != nil:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		resp.SetRetryAfterSeconds(retryAfterSeconds)
		return
	}

	// The reserved address must not be the Prism Central address. Prism will
	// happily reserve it, and the cluster would then come up having stolen the
	// address of the endpoint it manages itself.
	if err := assertNotPrismCentral(address, clusterConfig); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	// The address is deliberately not written back into the Cluster. NKP owns
	// the CAPI Cluster topology through its NKPCluster resource, and its webhook
	// rejects the patch outright:
	//
	//	spec.topology: Forbidden: CAPI Cluster topology spec is managed by
	//	NKPCluster ... and must not be modified directly.
	//
	// The reservation lives in the IPAddress instead, and the control plane
	// endpoint and virtual IP mutations read it from there on every topology
	// reconcile. Succeeding here is safe precisely because this hook does not
	// return success until the address exists, so the mutations always find one.
	log.Info("The API server address is reserved", "address", address)
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// errReservationPending means the provider has not allocated an address yet.
// It is not a failure; the hook is asked again.
var errReservationPending = errors.New("waiting for the API server address to be reserved")

// pendingOnClaim reports the wait and says where the reason lives.
//
// The hook retries until an address appears, so a reservation that can never
// succeed - an exhausted subnet pool answers kAddressPoolExhausted - looks
// identical to one that is merely slow. The IPAM provider does not currently
// record a condition when a reservation fails, so the cause is only in its
// logs; naming the claim at least points there, and any condition it does set
// is surfaced rather than swallowed.
func pendingOnClaim(claim *ipamv1.IPAddressClaim) error {
	key := ctrlclient.ObjectKeyFromObject(claim)
	for _, c := range claim.Status.Conditions {
		if c.Status != metav1.ConditionTrue && c.Message != "" {
			return fmt.Errorf("%w: IPAddressClaim %s: %s: %s",
				errReservationPending, key, c.Reason, c.Message)
		}
	}
	return fmt.Errorf("%w: IPAddressClaim %s has no address yet; "+
		"if this persists, check the IPAM provider logs for the subnet pool state",
		errReservationPending, key)
}

// reservationSource returns the reservation request and the Prism Central it
// should use, or nil when this cluster does not reserve its API server address.
func reservationSource(clusterConfig *apivariables.ClusterConfigSpec) (
	*carenv1.ControlPlaneIPReservation,
	*carenv1.NutanixPrismCentralEndpointSpec,
) {
	if clusterConfig == nil || clusterConfig.Nutanix == nil {
		return nil, nil
	}
	virtualIP := clusterConfig.Nutanix.ControlPlaneEndpoint.VirtualIPSpec
	if virtualIP == nil || virtualIP.Configuration == nil || virtualIP.Configuration.IPReservation == nil {
		return nil, nil
	}
	return virtualIP.Configuration.IPReservation, &clusterConfig.Nutanix.PrismCentralEndpoint
}

// reserve returns the address reserved for this cluster, creating the pool and
// the claim if they do not exist yet. It is safe to call repeatedly: both
// objects are named after the cluster, so a retry adopts what the last attempt
// created instead of reserving a second address.
func (h *ControlPlaneEndpointIPAM) reserve(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	endpoint *carenv1.NutanixPrismCentralEndpointSpec,
	reservation *carenv1.ControlPlaneIPReservation,
	log logr.Logger,
) (string, error) {
	name := poolName(cluster.Name)

	if err := h.ensurePool(ctx, cluster, name, endpoint, reservation, log); err != nil {
		return "", err
	}
	claim, err := h.ensureClaim(ctx, cluster, name, log)
	if err != nil {
		return "", err
	}

	if claim.Status.AddressRef.Name == "" {
		return "", pendingOnClaim(claim)
	}

	address := &ipamv1.IPAddress{}
	addressKey := ctrlclient.ObjectKey{Namespace: cluster.Namespace, Name: claim.Status.AddressRef.Name}
	if err := h.client.Get(ctx, addressKey, address); err != nil {
		if apierrors.IsNotFound(err) {
			// The claim points at an address that is not readable yet.
			return "", errReservationPending
		}
		return "", fmt.Errorf("%w: get IPAddress %s: %w", ErrAddressReservationFailed, addressKey, err)
	}
	if address.Spec.Address == "" {
		return "", errReservationPending
	}
	if _, err := netip.ParseAddr(address.Spec.Address); err != nil {
		return "", fmt.Errorf("%w: IPAddress %s holds %q, which is not an IP address",
			ErrAddressReservationFailed, addressKey, address.Spec.Address)
	}
	return address.Spec.Address, nil
}

func (h *ControlPlaneEndpointIPAM) ensurePool(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	name string,
	endpoint *carenv1.NutanixPrismCentralEndpointSpec,
	reservation *carenv1.ControlPlaneIPReservation,
	log logr.Logger,
) error {
	pool, err := newNutanixIPPool(name, cluster.Namespace, endpoint, reservation)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAddressReservationFailed, err)
	}
	pool.SetOwnerReferences([]metav1.OwnerReference{clusterOwnerReference(cluster)})

	existing := pool.DeepCopy()
	err = h.client.Get(ctx, ctrlclient.ObjectKeyFromObject(pool), existing)
	if err == nil {
		// The pool is left exactly as it is. Its subnet is immutable in this API,
		// and rewriting the Prism Central of a pool that already handed out an
		// address would point the release at the wrong endpoint.
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("%w: get NutanixIPPool %s/%s: %w",
			ErrAddressReservationFailed, cluster.Namespace, name, err)
	}

	log.Info("Creating NutanixIPPool for the API server address", "name", name)
	if err := h.client.Create(ctx, pool); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			// The NutanixIPPool kind does not exist, so the Nutanix IPAM provider
			// is not installed in this cluster. Retrying cannot fix that, and this
			// runs in the bootstrap cluster during a self-managed create, where a
			// retry loop would hang the create with no explanation.
			return fmt.Errorf("%w: %w", ErrIPAMProviderNotInstalled, err)
		}
		return fmt.Errorf("%w: create NutanixIPPool %s/%s: %w",
			ErrAddressReservationFailed, cluster.Namespace, name, err)
	}
	return nil
}

func (h *ControlPlaneEndpointIPAM) ensureClaim(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	name string,
	log logr.Logger,
) (*ipamv1.IPAddressClaim, error) {
	claim := &ipamv1.IPAddressClaim{}
	claimKey := ctrlclient.ObjectKey{Namespace: cluster.Namespace, Name: name}
	err := h.client.Get(ctx, claimKey, claim)
	if err == nil {
		return claim, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("%w: get IPAddressClaim %s: %w", ErrAddressReservationFailed, claimKey, err)
	}

	claim = &ipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
			// The owner reference is what releases the address: deleting the
			// cluster garbage-collects the claim, and the provider unreserves on
			// the claim's deletion.
			OwnerReferences: []metav1.OwnerReference{clusterOwnerReference(cluster)},
		},
		Spec: ipamv1.IPAddressClaimSpec{
			ClusterName: cluster.Name,
			PoolRef: ipamv1.IPPoolReference{
				APIGroup: nutanixIPPoolGVK.Group,
				Kind:     nutanixIPPoolGVK.Kind,
				Name:     name,
			},
		},
	}

	log.Info("Creating IPAddressClaim for the API server address", "name", name)
	if err := h.client.Create(ctx, claim); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if getErr := h.client.Get(ctx, claimKey, claim); getErr != nil {
				return nil, fmt.Errorf("%w: get IPAddressClaim %s: %w",
					ErrAddressReservationFailed, claimKey, getErr)
			}
			return claim, nil
		}
		return nil, fmt.Errorf("%w: create IPAddressClaim %s: %w",
			ErrAddressReservationFailed, claimKey, err)
	}
	return claim, nil
}

// assertNotPrismCentral refuses an address that is the Prism Central's own.
// The existing webhook compares the configured host against Prism Central, but
// it cannot see an address that does not exist yet, so this is the only place
// the check can be made for a reserved address.
func assertNotPrismCentral(address string, clusterConfig *apivariables.ClusterConfigSpec) error {
	if clusterConfig == nil || clusterConfig.Nutanix == nil {
		return nil
	}
	prismCentralIP, err := clusterConfig.Nutanix.PrismCentralEndpoint.ParseIP()
	if err != nil {
		// Prism Central is named by DNS, so there is nothing to compare.
		return nil
	}
	if prismCentralIP.String() == address {
		return fmt.Errorf(
			"%w: the reserved address %s is the Prism Central address; exclude it from the subnet's IP pool",
			ErrAddressReservationFailed, address,
		)
	}
	return nil
}

func clusterOwnerReference(cluster *clusterv1.Cluster) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: clusterv1.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       cluster.Name,
		UID:        cluster.UID,
	}
}
