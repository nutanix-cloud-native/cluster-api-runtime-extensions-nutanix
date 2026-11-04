// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	v1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

// storageNodePoolLabelKey marks both storage-only and compute-and-storage node pools.
const storageNodePoolLabelKey = "node-role.kubernetes.io/storage"

var machineDeploymentsPath = field.NewPath("spec", "topology", "workers", "machineDeployments")

type storageNodePoolValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewStorageNodePoolValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *storageNodePoolValidator {
	return &storageNodePoolValidator{
		client:  client,
		decoder: decoder,
	}
}

func (s *storageNodePoolValidator) Validator() admission.HandlerFunc {
	return s.validate
}

func (s *storageNodePoolValidator) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	if err := s.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !cluster.Spec.Topology.IsDefined() {
		return admission.Allowed("")
	}

	if provider := utils.GetProvider(cluster); provider != "nutanix" {
		return admission.Allowed("")
	}

	storagePools := storageNodePools(cluster)
	if len(storagePools) == 0 {
		return admission.Allowed("")
	}

	nutanixCSI, err := nutanixCSIConfigured(cluster)
	if err != nil {
		return admission.Denied(err.Error())
	}

	var oldCluster *clusterv1.Cluster
	if req.Operation == v1.Update {
		oldCluster = &clusterv1.Cluster{}
		if err := s.decoder.DecodeRaw(req.OldObject, oldCluster); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	fldErrs := append(
		validateAtMostOneStorageNodePool(storagePools, oldCluster),
		validateNoStorageNodePoolsWithNutanixCSI(storagePools, nutanixCSI, oldCluster)...,
	)
	if len(fldErrs) == 0 {
		return admission.Allowed("")
	}
	return deniedWithFieldErrors(cluster, fldErrs)
}

func validateAtMostOneStorageNodePool(storagePools []storagePool, oldCluster *clusterv1.Cluster) field.ErrorList {
	if len(storagePools) <= 1 {
		return nil
	}

	var kept, added []storagePool
	if oldCluster != nil {
		oldStoragePools := storageNodePools(oldCluster)
		// Clusters that already have multiple storage node pools must stay updatable, so only
		// reject updates that increase the count.
		if len(storagePools) <= len(oldStoragePools) {
			return nil
		}
		kept, added = splitKeptAndAdded(storagePools, oldStoragePools)
	}

	if len(kept) == 0 || len(added) == 0 {
		return multipleStorageNodePoolErrors(storagePools)
	}
	return addedStorageNodePoolErrors(added, kept)
}

func validateNoStorageNodePoolsWithNutanixCSI(
	storagePools []storagePool, nutanixCSI bool, oldCluster *clusterv1.Cluster,
) field.ErrorList {
	if !nutanixCSI {
		return nil
	}

	offending := storagePools
	if oldCluster != nil {
		// An old clusterConfig that can't be decoded is treated as already having Nutanix CSI, so only
		// newly added storage node pools are flagged. This only applies to updates that repair the
		// clusterConfig, because an undecodable clusterConfig on the new object is denied earlier.
		oldNutanixCSI, err := nutanixCSIConfigured(oldCluster)
		if err != nil || oldNutanixCSI {
			_, offending = splitKeptAndAdded(storagePools, storageNodePools(oldCluster))
		}
	}

	fldErrs := make(field.ErrorList, 0, len(offending))
	for _, pool := range offending {
		fldErrs = append(fldErrs, field.Forbidden(
			machineDeploymentsPath.Index(pool.index),
			fmt.Sprintf(
				"node pool %q is a storage node pool (label %q), which is not supported when the %q CSI "+
					"provider is configured; remove the label from the node pool",
				pool.name,
				storageNodePoolLabelKey,
				v1alpha1.CSIProviderNutanix,
			),
		))
	}
	return fldErrs
}

func nutanixCSIConfigured(cluster *clusterv1.Cluster) (bool, error) {
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal cluster topology variable %q: %w",
			v1alpha1.ClusterConfigVariableName, err)
	}
	if clusterConfig == nil || clusterConfig.Addons == nil || clusterConfig.Addons.CSI == nil {
		return false, nil
	}
	_, ok := clusterConfig.Addons.CSI.Providers[v1alpha1.CSIProviderNutanix]
	return ok, nil
}

type storagePool struct {
	index int
	name  string
}

func storageNodePools(cluster *clusterv1.Cluster) []storagePool {
	var pools []storagePool
	mds := cluster.Spec.Topology.Workers.MachineDeployments
	for i := range mds {
		if _, ok := mds[i].Metadata.Labels[storageNodePoolLabelKey]; ok {
			pools = append(pools, storagePool{index: i, name: mds[i].Name})
		}
	}
	return pools
}

// splitKeptAndAdded matches pools by name because their position in the list can change.
func splitKeptAndAdded(pools, oldPools []storagePool) (kept, added []storagePool) {
	wasStorage := make(map[string]bool, len(oldPools))
	for _, pool := range oldPools {
		wasStorage[pool.name] = true
	}
	for _, pool := range pools {
		if wasStorage[pool.name] {
			kept = append(kept, pool)
		} else {
			added = append(added, pool)
		}
	}
	return kept, added
}

func multipleStorageNodePoolErrors(pools []storagePool) field.ErrorList {
	fldErrs := make(field.ErrorList, 0, len(pools))
	for _, pool := range pools {
		fldErrs = append(fldErrs, field.Forbidden(
			machineDeploymentsPath.Index(pool.index),
			fmt.Sprintf(
				"node pool %q is one of %d storage node pools (label %q); at most one is supported "+
					"per cluster, remove the label from all but one node pool",
				pool.name,
				len(pools),
				storageNodePoolLabelKey,
			),
		))
	}
	return fldErrs
}

func addedStorageNodePoolErrors(added, kept []storagePool) field.ErrorList {
	keptNames := make([]string, 0, len(kept))
	for _, pool := range kept {
		keptNames = append(keptNames, strconv.Quote(pool.name))
	}
	existing := "storage node pool " + keptNames[0]
	if len(keptNames) > 1 {
		existing = "storage node pools " + strings.Join(keptNames, ", ")
	}

	fldErrs := make(field.ErrorList, 0, len(added))
	for _, pool := range added {
		fldErrs = append(fldErrs, field.Forbidden(
			machineDeploymentsPath.Index(pool.index),
			fmt.Sprintf(
				"node pool %q cannot be added as a storage node pool (label %q) because the cluster "+
					"already has %s; remove the label from %q or remove the node pool",
				pool.name,
				storageNodePoolLabelKey,
				existing,
				pool.name,
			),
		))
	}
	return fldErrs
}

func deniedWithFieldErrors(cluster *clusterv1.Cluster, fldErrs field.ErrorList) admission.Response {
	statusErr := apierrors.NewInvalid(
		clusterv1.GroupVersion.WithKind("Cluster").GroupKind(),
		cluster.Name,
		fldErrs,
	)
	return admission.Response{
		AdmissionResponse: v1.AdmissionResponse{
			Allowed: false,
			Result:  &statusErr.ErrStatus,
		},
	}
}
