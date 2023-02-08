// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/addons/clusterresourcesets"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/addons/fluxhelmrelease"
	k8sclient "github.com/d2iq-labs/capi-runtime-extensions/pkg/k8s/client"
)

type AddonProvider string

const (
	ClusterResourceSetAddonProvider AddonProvider = "ClusterResourceSet"
	FluxHelmReleaseAddonProvider    AddonProvider = "FluxHelmRelease"
)

// ExtensionHandlers provides a common struct shared across the lifecycle hook handlers.
type ExtensionHandlers struct {
	addonProvider AddonProvider
	client        ctrlclient.Client
}

// NewExtensionHandlers returns a ExtensionHandlers for the lifecycle hooks handlers.
func NewExtensionHandlers(
	addonProvider AddonProvider,
	client ctrlclient.Client,
) *ExtensionHandlers {
	return &ExtensionHandlers{
		addonProvider: addonProvider,
		client:        client,
	}
}

func (m *ExtensionHandlers) DoBeforeClusterCreate(
	ctx context.Context,
	request *runtimehooksv1.BeforeClusterCreateRequest,
	response *runtimehooksv1.BeforeClusterCreateResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterCreate is called")
}

func (m *ExtensionHandlers) DoAfterControlPlaneInitialized(
	ctx context.Context,
	request *runtimehooksv1.AfterControlPlaneInitializedRequest,
	response *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("AfterControlPlaneInitialized is called")

	genericResourcesClient := k8sclient.NewGenericResourcesClient(m.client, log)

	err := applyCNIResources(
		ctx,
		m.addonProvider,
		&request.Cluster,
		genericResourcesClient,
		m.client,
	)
	if err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
	}
}

func (m *ExtensionHandlers) DoBeforeClusterUpgrade(
	ctx context.Context,
	request *runtimehooksv1.BeforeClusterUpgradeRequest,
	response *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterUpgrade is called")
}

func (m *ExtensionHandlers) DoBeforeClusterDelete(
	ctx context.Context,
	request *runtimehooksv1.BeforeClusterDeleteRequest,
	response *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterDelete is called")

	genericResourcesClient := k8sclient.NewGenericResourcesClient(m.client, log)

	err := applyCNIResourcesForDelete(
		ctx,
		m.addonProvider,
		&request.Cluster,
		genericResourcesClient,
	)
	if err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
	}
}

func applyCNIResourcesForDelete(
	ctx context.Context,
	addonProvider AddonProvider,
	cluster *v1beta1.Cluster,
	genericResourcesClient *k8sclient.GenericResourcesClient,
) error {
	var (
		err  error
		objs []unstructured.Unstructured
	)
	switch addonProvider {
	case ClusterResourceSetAddonProvider:
		// Nothing to do.
	case FluxHelmReleaseAddonProvider:
		objs, err = fluxhelmrelease.CNIPatchesForClusterDelete(cluster)
	default:
		err = fmt.Errorf("unsupported provider: %q", addonProvider)
	}
	if err != nil {
		return err
	}

	return genericResourcesClient.Apply(ctx, objs...)
}

func applyCNIResources(
	ctx context.Context,
	addonProvider AddonProvider,
	cluster *v1beta1.Cluster,
	genericResourcesClient *k8sclient.GenericResourcesClient,
	c ctrlclient.Client,
) error {
	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		c,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return err
	}
	err = remoteClient.Patch(ctx, &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "tigera-operator",
			Labels: map[string]string{
				"pod-security.kubernetes.io/enforce":         "privileged",
				"pod-security.kubernetes.io/enforce-version": "latest",
			},
		},
	}, ctrlclient.Apply, ctrlclient.ForceOwnership, ctrlclient.FieldOwner("capi-runtime-extensions"))
	if err != nil {
		return err
	}

	var objs []unstructured.Unstructured
	switch addonProvider {
	case ClusterResourceSetAddonProvider:
		objs, err = clusterresourcesets.CNIForCluster(cluster)
	case FluxHelmReleaseAddonProvider:
		objs, err = fluxhelmrelease.CNIForCluster(cluster)
	default:
		err = fmt.Errorf("unsupported provider: %q", addonProvider)
	}
	if err != nil {
		return err
	}

	return genericResourcesClient.Apply(ctx, objs...)
}
