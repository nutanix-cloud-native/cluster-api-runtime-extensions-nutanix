package inclusterregistry

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

func (s *InClusterRegistryHandler) BeforeClusterCreate(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterCreateRequest,
	resp *runtimehooksv1.BeforeClusterCreateResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	s.setGlobalImageRegistryMirror(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

// Get a Service IP based on the Cluster's Service CIDR.
// Then set it back as the globalImageRegistryMirror variable.
func (s *InClusterRegistryHandler) setGlobalImageRegistryMirror(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	registryVar, err := variables.Get[v1alpha1.InClusterRegistry](
		varMap,
		s.variableName,
		s.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info(
					"Skipping InClusterRegistry, field is not specified",
					"error",
					err,
				)
			return
		}
		log.Error(
			err,
			"failed to read InClusterRegistry provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read InClusterRegistry provider from cluster definition: %v",
				err,
			),
		)
		return
	}

	handler, ok := s.ProviderHandler[registryVar.Provider]
	if !ok {
		err = fmt.Errorf("unknown InClusterRegistry Provider")
		log.Error(err, "provider", registryVar.Provider)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("%s %s", err, registryVar.Provider),
		)
		return
	}

	registryURL, err := handler.RegistryDetails(cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failde to get registry URL: %v", err))
	}

	globalImageRegistryMirror := &v1alpha1.GlobalImageRegistryMirror{
		URL: registryURL,
	}
	err = variables.Set(
		globalImageRegistryMirror, varMap,
		v1alpha1.ClusterConfigVariableName, v1alpha1.GlobalMirrorVariableName,
	)
	cluster.Spec.Topology.Variables = variables.VariablesMapToClusterVariables(varMap)

	err = s.client.Update(ctx, cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to set globalImageRegistryMirror variable on the Cluster: %v",
				err,
			),
		)
		return
	}
}
