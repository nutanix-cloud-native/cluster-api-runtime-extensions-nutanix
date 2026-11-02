// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package coredns

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
)

const handlerName = "CoreDNSFaultDomainSpread"

// errControlPlaneNodesPending is returned when not every expected control-plane
// node is Ready yet. AfterControlPlaneInitialized returns Failure so CAPI keeps
// the hook pending and retries after CP2/CP3 join; that is when metro clusters
// expose both NutanixMetroSite_* zones.
var errControlPlaneNodesPending = errors.New("waiting for remaining control-plane nodes")

// Handler patches the kubeadm-managed CoreDNS Deployment so replicas are required
// to land in different zones on multi-zone / metro clusters. Single-zone clusters
// are left unchanged.
type Handler struct {
	client              ctrlclient.Client
	clusterClientGetter remote.ClusterClientGetter
}

var (
	_ handlers.Named                         = &Handler{}
	_ lifecycle.AfterControlPlaneInitialized = &Handler{}
	_ lifecycle.AfterControlPlaneUpgrade     = &Handler{}
)

// New returns a CoreDNS fault-domain spread lifecycle handler.
func New(client ctrlclient.Client) *Handler {
	return &Handler{
		client:              client,
		clusterClientGetter: remote.NewClusterClient,
	}
}

func (h *Handler) Name() string {
	return handlerName
}

func (h *Handler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	// NKP marks the control plane initialized after CP1 is Ready, then joins
	// CP2/CP3. Returning Failure keeps this hook pending so we re-evaluate once
	// later control-plane nodes (and their zones) exist.
	h.apply(ctx, &req.Cluster, resp, true /* waitForControlPlane */)
}

func (h *Handler) AfterControlPlaneUpgrade(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneUpgradeRequest,
	resp *runtimehooksv1.AfterControlPlaneUpgradeResponse,
) {
	h.apply(ctx, &req.Cluster, resp, false /* waitForControlPlane */)
}

func (h *Handler) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp runtimehooksv1.ResponseObject,
	waitForControlPlane bool,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	log := ctrl.LoggerFrom(ctx).WithValues("cluster", clusterKey)

	remoteClient, err := h.clusterClientGetter(ctx, "", h.client, clusterKey)
	if err != nil {
		log.Error(err, "failed to create remote cluster client for CoreDNS spread")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to create remote cluster client: %v", err))
		return
	}

	err = ensureCoreDNSFaultDomainSpread(
		ctx,
		remoteClient,
		expectedControlPlaneReplicas(cluster),
		waitForControlPlane,
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("CoreDNS Deployment not found yet; will retry")
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage("coredns Deployment not found yet")
			return
		}
		if errors.Is(err, errControlPlaneNodesPending) {
			log.Info("waiting for remaining control-plane nodes before CoreDNS fault-domain spread")
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(err.Error())
			return
		}
		log.Error(err, "failed to ensure CoreDNS fault-domain spread")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func expectedControlPlaneReplicas(cluster *clusterv1.Cluster) int32 {
	if cluster.Spec.Topology.IsDefined() && cluster.Spec.Topology.ControlPlane.Replicas != nil {
		return *cluster.Spec.Topology.ControlPlane.Replicas
	}
	return 1
}

func ensureCoreDNSFaultDomainSpread(
	ctx context.Context,
	c ctrlclient.Client,
	expectedCPReplicas int32,
	waitForControlPlane bool,
) error {
	log := ctrl.LoggerFrom(ctx)

	var nodeList corev1.NodeList
	if err := c.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	if waitForControlPlane && expectedCPReplicas > 1 {
		readyCP := countReadyControlPlaneNodes(nodeList.Items)
		if readyCP < expectedCPReplicas {
			return fmt.Errorf(
				"%w: ready=%d expected=%d",
				errControlPlaneNodesPending,
				readyCP,
				expectedCPReplicas,
			)
		}
	}

	// Single-zone (or no zone labels): leave kubeadm CoreDNS defaults alone.
	if !isMultiZoneCluster(nodeList.Items) {
		log.V(4).Info("single-zone cluster; skipping CoreDNS fault-domain spread")
		return nil
	}

	desiredAntiAffinity := buildMultiZonePodAntiAffinity(nodeList.Items)

	deploy := &appsv1.Deployment{}
	key := types.NamespacedName{Name: DeploymentName, Namespace: DeploymentNamespace}
	if err := c.Get(ctx, key, deploy); err != nil {
		return err
	}

	var podList corev1.PodList
	if err := c.List(
		ctx,
		&podList,
		ctrlclient.InNamespace(DeploymentNamespace),
		ctrlclient.MatchingLabels{AppLabelKey: AppLabelValue},
	); err != nil {
		return fmt.Errorf("failed to list CoreDNS pods: %w", err)
	}

	needsAffinity := affinityNeedsUpdate(deploy.Spec.Template.Spec.Affinity, desiredAntiAffinity)
	// Affinity changes must force a new rollout annotation so kubeadm-scheduled pods move.
	forceRollout := needsAffinity || needsSpreadRollout(deploy.Spec.Template.Annotations, podList.Items, nodeList.Items)

	if !needsAffinity && !forceRollout {
		log.V(4).Info("CoreDNS affinity and spread already satisfy multi-zone policy")
		return nil
	}

	patchBase := deploy.DeepCopy()
	if needsAffinity {
		if deploy.Spec.Template.Spec.Affinity == nil {
			deploy.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		}
		deploy.Spec.Template.Spec.Affinity.PodAntiAffinity = desiredAntiAffinity
	}
	if forceRollout {
		if deploy.Spec.Template.Annotations == nil {
			deploy.Spec.Template.Annotations = map[string]string{}
		}
		deploy.Spec.Template.Annotations[spreadRolloutAnnotation] = spreadRolloutAnnotationValue
	}

	if err := c.Patch(ctx, deploy, ctrlclient.MergeFrom(patchBase)); err != nil {
		return fmt.Errorf("failed to patch CoreDNS Deployment for fault-domain spread: %w", err)
	}

	if needsAffinity {
		log.Info("patched CoreDNS Deployment with required zone pod anti-affinity")
	}
	if forceRollout {
		log.Info("triggered CoreDNS Deployment annotation rollout for fault-domain spread")
	}

	return nil
}
