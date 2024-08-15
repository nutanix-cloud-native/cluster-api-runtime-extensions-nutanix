// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	v1 "k8s.io/api/admission/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

type clusterUUIDLabeler struct {
	client  ctrlclient.Client
	decoder *admission.Decoder
}

func NewClusterUUIDLabeler(
	client ctrlclient.Client, decoder *admission.Decoder,
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
	cluster := &clusterv1.Cluster{}
	err := a.decoder.Decode(req, cluster)
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
		switch req.Operation {
		// If this is an update request, copy the UUID from the old object if it exists. This prevents deletion of the
		// annotation.
		//
		// This is also important for move operations where the clusterctl for some reason deletes
		// all annotations (see
		// https://github.com/kubernetes-sigs/cluster-api/blob/v1.7.4/cmd/clusterctl/client/cluster/mover.go#L1188).
		// Without this logic, the UUID would be deleted and the UUID validation webhook would fail.
		case v1.Update:
			oldCluster := &clusterv1.Cluster{}
			err := a.decoder.DecodeRaw(req.OldObject, oldCluster)
			if err != nil {
				return admission.Errored(
					http.StatusBadRequest,
					fmt.Errorf("failed to decode old cluster: %w", err),
				)
			}

			oldClusterUUID, ok := oldCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
			if ok {
				cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey] = oldClusterUUID
			}
		// If this is a create request, generate a new UUID.
		case v1.Create:
			cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey] = uuid.Must(uuid.NewV7()).
				String()
		}
	}

	marshaledCluster, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledCluster)
}

func (a *clusterUUIDLabeler) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	cluster := &clusterv1.Cluster{}
	err := a.decoder.Decode(req, cluster)
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
		oldCluster := &clusterv1.Cluster{}
		err := a.decoder.DecodeRaw(req.OldObject, oldCluster)
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
