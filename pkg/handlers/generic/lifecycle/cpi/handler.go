// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cpi

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	lifecycleutils "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/utils"
)

const (
	variableRootName = "cpi"
)

type CPIProvider interface {
	EnsureCPIConfigMapForCluster(context.Context, *clusterv1.Cluster) (*corev1.ConfigMap, error)
}

type CPIHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]CPIProvider
}

var (
	_ commonhandlers.Named                   = &CPIHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &CPIHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]CPIProvider,
) *CPIHandler {
	return &CPIHandler{
		client:          c,
		variableName:    clusterconfig.MetaVariableName,
		variablePath:    []string{"addons", variableRootName},
		ProviderHandler: handlers,
	}
}

func (c *CPIHandler) Name() string {
	return "CPIHandler"
}

func (c *CPIHandler) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(&req.Cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	varMap := variables.ClusterVariablesToVariablesMap(req.Cluster.Spec.Topology.Variables)

	_, found, err := variables.Get[v1alpha1.CPI](varMap, c.variableName, c.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read CPI from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CPI provider from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found {
		log.V(4).Info("Skipping CPI handler.")
		return
	}
	infraKind := req.Cluster.Spec.InfrastructureRef.Kind
	log.Info(fmt.Sprintf("finding cpi handler for %s", infraKind))
	var handler CPIProvider
	switch {
	case strings.Contains(strings.ToLower(infraKind), v1alpha1.CPIProviderAWS):
		handler = c.ProviderHandler[v1alpha1.CPIProviderAWS]
	default:
		log.Info(fmt.Sprintf("No CPI handler provided for infra kind %s", infraKind))
		return
	}
	cm, err := handler.EnsureCPIConfigMapForCluster(ctx, &req.Cluster)
	if err != nil {
		log.Error(
			err,
			"failed to generate CPI configmap",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to generate CPI configmap: %v",
				err,
			),
		)
		return
	}
	err = lifecycleutils.EnsureCRSForClusterFromConfigMaps(ctx, cm.Name, c.client, &req.Cluster, cm)
	if err != nil {
		log.Error(
			err,
			"failed to generate CPI CRS for cluster",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to generate CPI CRS: %v",
				err,
			),
		)
	}
}
