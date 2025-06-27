// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

type workloadClusterAutoEnabler struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewWorkloadClusterAutoEnabler(
	client ctrlclient.Client, decoder admission.Decoder,
) *workloadClusterAutoEnabler {
	return &workloadClusterAutoEnabler{
		client:  client,
		decoder: decoder,
	}
}

func (a *workloadClusterAutoEnabler) Defaulter() admission.HandlerFunc {
	return a.defaulter
}

func (a *workloadClusterAutoEnabler) defaulter(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	cluster := &clusterv1.Cluster{}
	err := a.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if hasSkipAnnotation(cluster) {
		return admission.Allowed("")
	}

	if cluster.Spec.Topology == nil {
		return admission.Allowed("")
	}

	// Check if the addon is already enabled in the cluster, if it is just return.
	clusterRegistry, err := registryAddonFromCluster(cluster)
	if err != nil {
		return admission.Errored(
			http.StatusInternalServerError,
			fmt.Errorf(
				"failed to check if registry addon is enabled in cluster: %w",
				err,
			))
	}
	if clusterRegistry != nil {
		return admission.Allowed("")
	}

	// Check if the global image registry mirror is enabled in the cluster, if it is just return.
	globalImageRegistryMirror, err := globalImageRegistryMirrorFromCluster(cluster)
	if err != nil {
		return admission.Errored(
			http.StatusInternalServerError,
			fmt.Errorf(
				"failed to check if global image registry mirror is enabled in cluster: %w",
				err,
			))
	}
	if globalImageRegistryMirror != nil {
		return admission.Allowed("")
	}

	managementCluster, err := capiutils.ManagementCluster(ctx, a.client)
	if err != nil {
		return admission.Errored(
			http.StatusInternalServerError,
			fmt.Errorf(
				"failed to get management cluster: %w",
				err,
			))
	}
	// Check if creating a workload cluster, ie managementCluster is not nil. If it is nil just return.
	if managementCluster == nil {
		return admission.Allowed("")
	}
	// Check if the addon is enabled in the management cluster, if not just return.
	managementClusterRegistry, err := registryAddonFromCluster(managementCluster)
	if err != nil {
		return admission.Errored(
			http.StatusInternalServerError,
			fmt.Errorf(
				"failed to check if registry addon is enabled in management cluster: %w",
				err,
			))
	}
	if managementClusterRegistry == nil {
		return admission.Allowed("")
	}

	// If the registry addon is not enabled in the cluster and is enabled in the management cluster, enable it here.
	err = enabledSameRegistryAddonInCluster(cluster, managementClusterRegistry)
	if err != nil {
		return admission.Errored(
			http.StatusInternalServerError,
			fmt.Errorf(
				"failed to enable registry addon in cluster: %w",
				err,
			))
	}

	marshaledCluster, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledCluster)
}

func hasSkipAnnotation(cluster *clusterv1.Cluster) bool {
	if cluster.Annotations == nil {
		return false
	}
	_, ok := cluster.Annotations[carenv1.SkipAutoEnablingWorkloadClusterRegistry]
	return ok
}

func registryAddonFromCluster(cluster *clusterv1.Cluster) (*carenv1.RegistryAddon, error) {
	spec, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster variable: %w", err)
	}
	if spec == nil {
		return nil, nil
	}
	if spec.Addons == nil {
		return nil, nil
	}

	return spec.Addons.Registry, nil
}

func globalImageRegistryMirrorFromCluster(cluster *clusterv1.Cluster) (*carenv1.GlobalImageRegistryMirror, error) {
	spec, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster variable: %w", err)
	}
	if spec == nil {
		return nil, nil
	}

	return spec.GlobalImageRegistryMirror, nil
}

func enabledSameRegistryAddonInCluster(cluster *clusterv1.Cluster, sourceAddon *carenv1.RegistryAddon) error {
	spec, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cluster variable: %w", err)
	}

	if spec.Addons == nil {
		spec.Addons = &variables.Addons{}
	}
	spec.Addons.Registry = &carenv1.RegistryAddon{
		Provider: sourceAddon.Provider,
	}

	variable, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, spec)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster variable: %w", err)
	}
	cluster.Spec.Topology.Variables = variables.UpdateClusterVariable(variable, cluster.Spec.Topology.Variables)

	return nil
}
