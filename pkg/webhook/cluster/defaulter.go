// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// clusterMutator applies all cluster mutating defaults in one pass and returns a single JSON patch.
// This avoids admission.MultiMutatingHandler, which concatenates patches computed independently from the
// same original object: two handlers that each add /metadata/annotations as a single-key object would
// overwrite one another, dropping e.g. caren.nutanix.com/cluster-uuid before validation.
type clusterMutator struct {
	decoder  admission.Decoder
	uuid     *clusterUUIDLabeler
	policy   *projectPolicyDefaulter
	registry *nutanixRegistryDefaulter
}

func NewDefaulter(client ctrlclient.Client, decoder admission.Decoder) admission.Handler {
	return &clusterMutator{
		decoder:  decoder,
		uuid:     NewClusterUUIDLabeler(client, decoder),
		policy:   NewProjectPolicyDefaulter(client),
		registry: NewNutanixRegistryDefaulter(client, decoder),
	}
}

func (m *clusterMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &clusterv1.Cluster{}
	if err := m.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := m.uuid.mutateCluster(req, cluster); err != nil {
		if errors.Is(err, errSkipClusterMutation) {
			return admission.Allowed("")
		}
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("cluster UUID defaulting: %w", err))
	}

	if err := m.policy.mutateCluster(ctx, req, cluster); err != nil {
		return admission.Errored(
			http.StatusInternalServerError,
			fmt.Errorf("cluster project policy defaulting: %w", err),
		)
	}

	if err := m.registry.mutateCluster(ctx, req, cluster); err != nil {
		return admission.Errored(
			http.StatusInternalServerError,
			fmt.Errorf("nutanix registry credentials defaulting: %w", err),
		)
	}

	marshaledCluster, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledCluster)
}
