// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	v1 "k8s.io/api/admission/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook"
)

type clusterUUIDLabeler struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewClusterUUIDLabeler(
	client ctrlclient.Client, decoder admission.Decoder,
) *clusterUUIDLabeler {
	return &clusterUUIDLabeler{
		client:  client,
		decoder: decoder,
	}
}

func (a *clusterUUIDLabeler) Defaulter() admission.HandlerFunc {
	return a.defaulter
}

func (a *clusterUUIDLabeler) Validator() admission.HandlerFunc {
	return a.validate
}

func (a *clusterUUIDLabeler) defaulter(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	cluster, err := webhook.DecodeCluster(a.decoder, req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil {
		return admission.Allowed("")
	}

	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string, 1)
	}

	// Only manipulate the UUID annotation if it is not already set.
	if _, ok := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]; !ok {
		var newUUID string
		switch req.Operation {
		// If this is an update request, copy the UUID from the old object if it exists. This prevents deletion of the
		// annotation.
		//
		// This is especially important for move operations where the clusterctl for some reason deletes
		// all annotations (see
		// https://github.com/kubernetes-sigs/cluster-api/blob/v1.7.4/cmd/clusterctl/client/cluster/mover.go#L1188).
		// Without this logic, the UUID would be deleted and the UUID validation webhook would fail.
		case v1.Update:
			oldCluster, err := webhook.DecodeClusterRaw(a.decoder, req.OldObject)
			if err != nil {
				return admission.Errored(
					http.StatusBadRequest,
					fmt.Errorf("failed to decode old cluster: %w", err),
				)
			}

			oldClusterUUID, ok := oldCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
			if ok {
				newUUID = oldClusterUUID
			} else {
				// If the old cluster does not have a UUID, generate a new one. This can happen if the cluster was
				// created before this webhook was installed or if the cluster began without spec.topology and was
				// later converted to a ClusterClass based cluster.
				newUUID = uuid.Must(uuid.NewV7()).String()
			}
		// If this is a create request, generate a new UUID.
		case v1.Create:
			newUUID = uuid.Must(uuid.NewV7()).String()
		}

		// Use raw JSON patching to preserve all v1beta2 fields
		return webhook.PatchRawAnnotation(req.Object.Raw, v1alpha1.ClusterUUIDAnnotationKey, newUUID)
	}

	return admission.Allowed("")
}

func (a *clusterUUIDLabeler) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	cluster, err := webhook.DecodeCluster(a.decoder, req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil {
		return admission.Allowed("")
	}

	clusterUUID, ok := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
	if !ok {
		return admission.Denied(
			fmt.Sprintf(
				"missing cluster UUID annotation %s",
				v1alpha1.ClusterUUIDAnnotationKey,
			),
		)
	}

	if req.Operation == v1.Update {
		oldCluster, err := webhook.DecodeClusterRaw(a.decoder, req.OldObject)
		if err != nil {
			return admission.Errored(
				http.StatusBadRequest,
				fmt.Errorf("failed to decode old cluster: %w", err),
			)
		}

		oldClusterUUID, ok := oldCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
		if !ok {
			return admission.Allowed("")
		}

		if clusterUUID != oldClusterUUID {
			return admission.Denied(
				fmt.Sprintf(
					"cluster UUID annotation %s is immutable and cannot be changed from %s to %s",
					v1alpha1.ClusterUUIDAnnotationKey,
					oldClusterUUID,
					clusterUUID,
				),
			)
		}
	}

	return admission.Allowed("")
}
