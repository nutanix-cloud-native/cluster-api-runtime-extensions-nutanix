// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/helpers"
)

type metroCSIComputeAffinity struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewMetroCSIComputeAffinity(
	client ctrlclient.Client, decoder admission.Decoder,
) *metroCSIComputeAffinity {
	return &metroCSIComputeAffinity{
		client:  client,
		decoder: decoder,
	}
}

func (a *metroCSIComputeAffinity) Defaulter() admission.HandlerFunc {
	return a.defaulter
}

func (a *metroCSIComputeAffinity) Validator() admission.HandlerFunc {
	return a.validate
}

func (a *metroCSIComputeAffinity) defaulter(
	_ context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	if err := a.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !cluster.Spec.Topology.IsDefined() {
		return admission.Allowed("")
	}

	mutated, err := helpers.DefaultMetroCSIComputeAffinity(cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if !mutated {
		return admission.Allowed("")
	}

	marshaledCluster, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledCluster)
}

func (a *metroCSIComputeAffinity) validate(
	_ context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	if err := a.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !cluster.Spec.Topology.IsDefined() {
		return admission.Allowed("")
	}

	if err := helpers.ValidateMetroCSIComputeAffinity(cluster); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}
