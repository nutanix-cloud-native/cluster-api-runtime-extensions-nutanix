// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ccm

import (
	"context"
	"fmt"
	"strings"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	variableRootName = "ccm"
)

type CCMProvider interface {
	Apply(context.Context, *clusterv1.Cluster) error
}

type CCMHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]CCMProvider
}

var (
	_ commonhandlers.Named                   = &CCMHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &CCMHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]CCMProvider,
) *CCMHandler {
	return &CCMHandler{
		client:          c,
		variableName:    clusterconfig.MetaVariableName,
		variablePath:    []string{"addons", variableRootName},
		ProviderHandler: handlers,
	}
}

func (c *CCMHandler) Name() string {
	return "CCMHandler"
}

func (c *CCMHandler) AfterControlPlaneInitialized(
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

	_, found, err := variables.Get[v1alpha1.CCM](varMap, c.variableName, c.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read CCM from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CCM provider from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found {
		log.V(4).Info("Skipping CCM handler.")
		return
	}
	infraKind := req.Cluster.Spec.InfrastructureRef.Kind
	log.Info(fmt.Sprintf("finding CCM handler for %s", infraKind))
	var handler CCMProvider
	switch {
	case strings.Contains(strings.ToLower(infraKind), v1alpha1.CCMProviderAWS):
		handler = c.ProviderHandler[v1alpha1.CCMProviderAWS]
	default:
		log.Info(fmt.Sprintf("No CCM handler provided for infra kind %s", infraKind))
		return
	}
	err = handler.Apply(ctx, &req.Cluster)
	if err != nil {
		log.Error(
			err,
			"failed to deploy CCM for cluster",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to deploy CCM for cluster: %v",
				err,
			),
		)
		return
	}
}
