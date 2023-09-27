package csi

import (
	"context"
	"fmt"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	variableRootName = "csiProviders"
)

type CSIProvider interface {
	EnsureCSIConfigMapForCluster(context.Context, *clusterv1.Cluster) (*corev1.ConfigMap, error)
	EnsureCSICRSForCluster(context.Context, *clusterv1.Cluster, *corev1.ConfigMap) error
}

type CSIHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]CSIProvider
}

var (
	_ commonhandlers.Named                   = &CSIHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &CSIHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]CSIProvider,
) *CSIHandler {
	return &CSIHandler{
		client:          c,
		variableName:    clusterconfig.MetaVariableName,
		variablePath:    []string{"addons", variableRootName},
		ProviderHandler: handlers,
	}
}

func (a *CSIHandler) Name() string {
	return "AWSEBS"
}

func (a *CSIHandler) AfterControlPlaneInitialized(
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
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	csiProviders, found, err := variables.Get[v1alpha1.CSIProviders](varMap, a.variableName, a.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read CSI provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CSI provider from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found || csiProviders.Providers == nil || len(csiProviders.Providers) == 0 {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping CSI handler, no providers given in %v",
				csiProviders,
			),
		)
		return
	}
	for _, provider := range csiProviders.Providers {
		handler, ok := a.ProviderHandler[provider.Name]
		if !ok {
			log.V(4).Info(
				fmt.Sprintf(
					"Skipping CSI handler, for provider given in %q. Provider handler not given ",
					provider,
				),
			)
			continue
		}
		cm, err := handler.EnsureCSIConfigMapForCluster(ctx, &req.Cluster)
		if err != nil {
			log.Error(
				err,
				fmt.Sprintf("failed to ensure %s csi driver installation manifests ConfigMap", provider.Name),
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		}
		err = handler.EnsureCSICRSForCluster(ctx, &req.Cluster, cm)
		if err != nil {
			log.Error(
				err,
				fmt.Sprintf("failed to ensure %s csi driver installation manifests ConfigMap", provider.Name),
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		}
	}
}
