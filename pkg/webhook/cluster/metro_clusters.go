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

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/metro"
)

type metroClusters struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewMetroClustersDefaulter(
	client ctrlclient.Client, decoder admission.Decoder,
) *metroClusters {
	return &metroClusters{
		client:  client,
		decoder: decoder,
	}
}

func (a *metroClusters) Defaulter() admission.HandlerFunc {
	return a.defaulter
}

func (a *metroClusters) Validator() admission.HandlerFunc {
	return a.validate
}

func (a *metroClusters) defaulter(
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

	if !cluster.Spec.Topology.IsDefined() || !metro.IsMetroCluster(cluster) {
		return admission.Allowed("")
	}

	if err := metro.DefaultCSIComputeAffinity(cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	marshaledCluster, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledCluster)
}

func (a *metroClusters) validate(
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

	if !cluster.Spec.Topology.IsDefined() || !metro.IsMetroCluster(cluster) {
		return admission.Allowed("")
	}

	if err := metro.ValidateCSIComputeAffinity(cluster); err != nil {
		return admission.Denied(err.Error())
	}
	return admission.Allowed("")
}
