// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ipamv1 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
)

type ServiceLoadBalancerGC struct {
	client              ctrlclient.Client
	clusterClientGetter remote.ClusterClientGetter
	// releaserGetter builds the Prism Central client the reservation sweep uses.
	// It is a field so tests can supply a fake without a Prism Central.
	releaserGetter releaserGetter
}

var (
	_ handlers.Named                = &ServiceLoadBalancerGC{}
	_ lifecycle.BeforeClusterDelete = &ServiceLoadBalancerGC{}
)

func New(client ctrlclient.Client) *ServiceLoadBalancerGC {
	return &ServiceLoadBalancerGC{
		client:              client,
		clusterClientGetter: remote.NewClusterClient,
		releaserGetter: func(ctx context.Context, namespace string, endpoint *carenv1.NutanixPrismCentralEndpointSpec) (reservationReleaser, error) {
			return newReservationReleaser(ctx, client, namespace, endpoint)
		},
	}
}

// releaserGetter builds a Prism Central client for a cluster's credentials.
type releaserGetter func(
	ctx context.Context,
	namespace string,
	endpoint *carenv1.NutanixPrismCentralEndpointSpec,
) (reservationReleaser, error)

func (s *ServiceLoadBalancerGC) Name() string {
	return "ServiceLoadBalancerGC"
}

func (s *ServiceLoadBalancerGC) BeforeClusterDelete(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	cluster := &req.Cluster
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	// CAPI 1.12+ strips cluster status from the hook request. Fetch the full Cluster from the API
	// so we have phase and conditions for the cleanup decision (shouldDeleteServicesWithLoadBalancer).
	clusterWithStatus := &clusterv1.Cluster{}
	if err := s.client.Get(ctx, clusterKey, clusterWithStatus); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Cluster not found (may already be deleted), allowing deletion")
			resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
			return
		}
		log.Error(err, "Failed to get cluster with status for Service LB GC decision, will retry")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to get cluster with status: %v", err))
		resp.SetRetryAfterSeconds(5)
		return
	}

	shouldDelete, err := shouldDeleteServicesWithLoadBalancer(clusterWithStatus)
	if err != nil {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf(
			"error determining if Services of type LoadBalancer should be deleted: %v",
			err,
		)
		return
	}

	if !shouldDelete {
		// Deleting the Services was skipped, so the CCM was never asked to release
		// their addresses. Release them here instead: this is the path taken when
		// the control plane is unreachable, which is the most common reason anyone
		// deletes a cluster. The one exception is an explicit opt-out, which is the
		// only signal an operator has for "do not clean up my load balancer
		// resources", and it is honoured for Prism Central too.
		if !loadBalancerGCEnabled(clusterWithStatus) {
			log.Info("Skipping load balancer cleanup entirely",
				"annotation", LoadBalancerGCAnnotation)
			resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
			return
		}
		s.releaseReservations(ctx, cluster, resp, log)
		return
	}

	log.Info("Will attempt to delete Services with type LoadBalancer")
	remoteClient, err := s.clusterClientGetter(
		ctx,
		"",
		s.client,
		clusterKey,
	)
	if err != nil {
		// The workload cluster cannot be reached, so its Services cannot be
		// deleted and the CCM will never be asked to release their addresses.
		// This is reached for the commonest teardown of all: a cluster whose
		// control plane has died. ClusterControlPlaneInitializedCondition records
		// that the control plane came up once and stays true afterwards, so
		// shouldDeleteServicesWithLoadBalancer sends a dead cluster down this
		// path rather than the skip path above.
		//
		// Release the addresses before reporting the failure. The response is
		// unchanged, so deletion still blocks exactly as it did before; this only
		// makes sure nothing is left reserved on Prism Central when an operator
		// eventually forces the deletion through.
		if sweepErr := s.sweepReservations(ctx, cluster, log); sweepErr != nil {
			log.Error(sweepErr, "Failed to release reservations for an unreachable cluster")
		}
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf(
			"error creating remote cluster client: %v",
			err,
		)
		return
	}

	err = deleteServicesWithLoadBalancer(ctx, remoteClient, log)
	switch {
	case errors.Is(err, ErrFailedToDeleteService), errors.Is(err, ErrServicesStillExist):
		// The Services are still there, so the CCM may still be holding and
		// advertising their addresses. Sweeping Prism now would unreserve an
		// address MetalLB is still announcing, which is the defect the sweep
		// exists to prevent. Retry and sweep once they are gone.
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		resp.SetRetryAfterSeconds(5)
		return
	case err != nil:
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	// Every LoadBalancer Service is gone, so the CCM has released what it knew
	// about. Anything left is a leak.
	s.releaseReservations(ctx, cluster, resp, log)
}

// releaseReservations releases the addresses Prism Central still holds for the
// cluster. It runs in this handler rather than its own because the aggregated
// BeforeClusterDelete hook runs its handlers in parallel, and this step must
// follow the Service deletion above instead of racing it.
func (s *ServiceLoadBalancerGC) releaseReservations(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
	log logr.Logger,
) {
	subnet, prismElementCluster, endpoint, err := autoIPAMReservationSource(cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}
	if subnet == "" {
		// Static address ranges: nothing was ever reserved on Prism Central.
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		return
	}

	if s.releaserGetter == nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("%v: no Prism Central client is configured", ErrFailedToReleaseReservations))
		return
	}

	releaser, err := s.releaserGetter(ctx, cluster.Namespace, endpoint)
	if err != nil {
		log.Error(err, "Failed to build a Prism Central client for the reservation sweep")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("%v: %v", ErrFailedToReleaseReservations, err))
		resp.SetRetryAfterSeconds(5)
		return
	}

	claims := s.controlPlaneClaimContexts(ctx, cluster, log)
	if err := releaseClusterReservations(
		ctx, releaser, cluster, subnet, prismElementCluster, claims, log,
	); err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		resp.SetRetryAfterSeconds(5)
		return
	}
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// controlPlaneClaimContexts returns the reservation contexts the Nutanix IPAM
// provider wrote for this cluster's address claims, which is each claim's own
// UID. They have to be read here, while the claims still exist: once the
// Cluster is gone they are garbage-collected with it, and a bare UID left on
// Prism Central cannot be attributed to any cluster afterwards.
//
// Failure is logged and not returned. The provider releases these addresses
// itself when the claim is deleted, so this is the safety net for the case
// where it cannot: it must never block deletion.
func (s *ServiceLoadBalancerGC) controlPlaneClaimContexts(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) []string {
	claims := &ipamv1.IPAddressClaimList{}
	if err := s.client.List(ctx, claims,
		ctrlclient.InNamespace(cluster.Namespace),
		ctrlclient.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
	); err != nil {
		log.Error(err, "Could not list IPAddressClaims; the API server address may be left reserved")
		return nil
	}
	contexts := make([]string, 0, len(claims.Items))
	for i := range claims.Items {
		if uid := string(claims.Items[i].GetUID()); uid != "" {
			contexts = append(contexts, uid)
		}
	}
	return contexts
}

// sweepReservations releases the cluster's reservations without touching the
// hook response, for callers that are already reporting a different outcome.
func (s *ServiceLoadBalancerGC) sweepReservations(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	subnet, prismElementCluster, endpoint, err := autoIPAMReservationSource(cluster)
	if err != nil || subnet == "" {
		return err
	}
	if s.releaserGetter == nil {
		return fmt.Errorf("%w: no Prism Central client is configured", ErrFailedToReleaseReservations)
	}
	releaser, err := s.releaserGetter(ctx, cluster.Namespace, endpoint)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToReleaseReservations, err)
	}
	return releaseClusterReservations(
		ctx, releaser, cluster, subnet, prismElementCluster,
		s.controlPlaneClaimContexts(ctx, cluster, log), log,
	)
}

// autoIPAMReservationSource returns the subnet the cluster reserves Service
// addresses from, the Prism Element cluster that resolves it by name, and the
// Prism Central to talk to. An empty subnet means the cluster does not reserve
// addresses at all.
func autoIPAMReservationSource(cluster *clusterv1.Cluster) (
	subnet string,
	prismElementCluster string,
	endpoint *carenv1.NutanixPrismCentralEndpointSpec,
	err error,
) {
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: read cluster config: %w", ErrFailedToReleaseReservations, err)
	}
	if clusterConfig == nil || clusterConfig.Nutanix == nil || clusterConfig.Addons == nil {
		return "", "", nil, nil
	}
	slb := clusterConfig.Addons.ServiceLoadBalancer
	if slb == nil || slb.Configuration == nil || slb.Configuration.AutoIPAM == nil {
		return "", "", nil, nil
	}
	autoIPAM := slb.Configuration.AutoIPAM
	if autoIPAM.Cluster != nil {
		prismElementCluster = *autoIPAM.Cluster
	}
	return autoIPAM.Subnet, prismElementCluster, &clusterConfig.Nutanix.PrismCentralEndpoint, nil
}
