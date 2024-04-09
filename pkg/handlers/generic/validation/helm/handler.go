// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

type HelmRegistryValidator struct {
	client       ctrlclient.Client
	variableName string
}

func New(
	c ctrlclient.Client,
) *HelmRegistryValidator {
	return &HelmRegistryValidator{
		client:       c,
		variableName: clusterconfig.MetaVariableName,
	}
}

func (h *HelmRegistryValidator) Name() string {
	return "HelmRegistryValidator"
}

func (h *HelmRegistryValidator) ValidateTopology(
	ctx context.Context,
	req *runtimehooksv1.ValidateTopologyRequest,
	res *runtimehooksv1.ValidateTopologyResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	clusterVar, ind := variables.GetRuntimhookVariableByName(h.variableName, req.Variables)
	if ind == -1 {
		log.V(5).Info(fmt.Sprintf("did not find variable %s in %v", h.variableName, req.Variables))
		return
	}
	var cluster v1alpha1.ClusterConfig
	if err := variables.UnmarshalRuntimeVariable[v1alpha1.ClusterConfig](clusterVar, &cluster); err != nil {
		failString := fmt.Sprintf("failed  to unmarshal variable %v to clusterConfig", clusterVar)
		log.Error(err, failString)
		res.SetStatus(runtimehooksv1.ResponseStatusFailure)
		res.SetMessage(failString)
		return
	}
	helmChartRepo := cluster.Spec.Addons.HelmChartRepository
	cl := &http.Client{
		Transport: &http.Transport{
			//nolint:gosec // this is done because customers can occasionally have self signed
			// or no certificates to OCI registries
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := cl.Get(fmt.Sprintf("%s/v2", *helmChartRepo))
	if err != nil {
		failString := fmt.Sprintf("failed to ping provided helm registry %s", *helmChartRepo)
		log.Error(err, failString)
		res.SetStatus(runtimehooksv1.ResponseStatusFailure)
		res.SetMessage(failString)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		res.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		return
	}
	failString := fmt.Sprintf(
		"failed to get 401 or 200 response from hitting registry: %s got status: %d",
		*helmChartRepo,
		resp.StatusCode,
	)
	log.Error(err, failString)
	res.SetStatus(runtimehooksv1.ResponseStatusFailure)
	res.SetMessage(failString)
}
