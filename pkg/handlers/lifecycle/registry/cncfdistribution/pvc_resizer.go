// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cncfdistribution

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/registry/utils"
)

// updateStatefulSetVolumeClaimTemplate ensures that the registry StatefulSet volumeClaimTemplates storage request
// matches the desired persistence size from the Helm values.
// If the storage request is different, the StatefulSet will be deleted and it is expected that will be recreated
// with the new volumeClaimTemplates by Helm.
// When deleting, it is using the equivalent of "kubectl delete sts --cascade=orphan"
// to ensure that the PVCs are not deleted.
//
// This function is defensive and only deletes the StatefulSet when needed.
func updateStatefulSetVolumeClaimTemplate(
	ctx context.Context,
	_ ctrlclient.Client,
	remoteClient ctrlclient.Client,
	cluster *clusterv1.Cluster,
	hcp *caaphv1.HelmChartProxy,
) error {
	// Get the expected persistence size from the Helm values.
	expectedPersistenceSize, err := expectedPersistenceSize(hcp)
	if err != nil {
		return fmt.Errorf("failed to get expected persistence size from Helm values: %w", err)
	}
	// Return early if there is no expected persistence size.
	if expectedPersistenceSize == "" {
		return nil
	}

	// Get expected registry metadata from the cluster, ie. namespace, statefulset name, etc.
	registryMetadata, err := utils.GetRegistryMetadata(cluster)
	if err != nil {
		return fmt.Errorf("failed to get registry metadata: %w", err)
	}

	// Get the StatefulSet from the remote cluster.
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registryMetadata.StatefulSetName,
			Namespace: registryMetadata.Namespace,
		},
	}
	err = remoteClient.Get(ctx, ctrlclient.ObjectKeyFromObject(sts), sts)
	if err != nil {
		// If the StatefulSet doesn't exist, there's nothing to update.
		// Helm will create it with the correct volumeClaimTemplates.
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get StatefulSet: %w", err)
	}

	// Get the current persistence size from the StatefulSet volumeClaimTemplates.
	// It is assumed that the first volumeClaimTemplate is the one that is used for the persistence.
	volumeClaimTemplates := sts.Spec.VolumeClaimTemplates
	if len(volumeClaimTemplates) == 0 {
		return nil
	}
	currentPersistenceSize := volumeClaimTemplates[0].Spec.Resources.Requests.Storage().String()

	// Return early if the current persistence size already matches the expected persistence size.
	if currentPersistenceSize == expectedPersistenceSize {
		return nil
	}

	// Delete the StatefulSet with the orphan propagation policy to ensure that the PVCs are not deleted.
	deleteOpts := &client.DeleteOptions{
		PropagationPolicy: ptr.To(metav1.DeletePropagationOrphan),
	}
	err = remoteClient.Delete(ctx, sts, deleteOpts)
	if err != nil {
		return fmt.Errorf("failed to delete StatefulSet %s/%s: %w", sts.Namespace, sts.Name, err)
	}

	// Just return and let Helm recreate the StatefulSet with the new volumeClaimTemplates.
	return nil
}

// expandPersistentVolumeClaims expands the PVC disk size of the registry StatefulSet.
// The volumeClaimTemplates field of a StatefulSet is immutable, so changing the Helm values is not enough.
// Instead, this function will compare the expected disk size in the Helm values with the StatefulSet volumeClaimTemplates size.
// If the storage request is different, the PVC will be resized to the expected persistence size.
//
// This function is defensive and only resizes the PVCs when needed.
func expandPersistentVolumeClaims(
	ctx context.Context,
	_ ctrlclient.Client,
	remoteClient ctrlclient.Client,
	cluster *clusterv1.Cluster,
	hcp *caaphv1.HelmChartProxy,
) error {
	// Get the expected persistence size from the Helm values.
	expectedPersistenceSize, err := expectedPersistenceSize(hcp)
	if err != nil {
		return fmt.Errorf("failed to get expected persistence size from Helm values: %w", err)
	}
	// Return early if there is no expected persistence size.
	if expectedPersistenceSize == "" {
		return nil
	}

	// Get expected registry metadata from the cluster, ie. namespace, statefulset name, etc.
	registryMetadata, err := utils.GetRegistryMetadata(cluster)
	if err != nil {
		return fmt.Errorf("failed to get registry metadata: %w", err)
	}

	// Get the PVCs from the remote cluster that are associated with the StatefulSet.
	pvcs := &corev1.PersistentVolumeClaimList{}
	err = remoteClient.List(ctx, pvcs, &client.ListOptions{
		Namespace: registryMetadata.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"release": registryMetadata.HelmReleaseName,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to list PVCs: %w", err)
	}

	if len(pvcs.Items) == 0 {
		return nil
	}

	expectedPersistenceSizeQuantity, err := resource.ParseQuantity(expectedPersistenceSize)
	if err != nil {
		return fmt.Errorf("failed to parse expected persistence size: %w", err)
	}

	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		// Skip PVCs that don't have a storage request set.
		if pvc.Spec.Resources.Requests == nil {
			continue
		}
		storageRequest := pvc.Spec.Resources.Requests.Storage()
		if storageRequest == nil {
			continue
		}

		// Skip PVCs that already have the expected persistence size.
		if storageRequest.Cmp(expectedPersistenceSizeQuantity) == 0 {
			continue
		}

		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = expectedPersistenceSizeQuantity
		if err := remoteClient.Update(ctx, pvc); err != nil {
			return fmt.Errorf("failed to update PVC %s/%s: %w", pvc.Namespace, pvc.Name, err)
		}
	}

	return nil
}

// expectedPersistenceSize returns the expected persistence size from the Helm values.
// If the persistence is not enabled, it returns an empty string.
func expectedPersistenceSize(hcp *caaphv1.HelmChartProxy) (string, error) {
	// Return early if there are no values to process.
	values := hcp.Spec.ValuesTemplate
	if values == "" {
		return "", nil
	}

	// Parse the values YAML into a generic map.
	var valuesObj map[string]interface{}
	if err := yaml.Unmarshal([]byte(values), &valuesObj); err != nil {
		return "", fmt.Errorf("failed to parse values: %w", err)
	}

	// Return early if the persistence is not enabled.
	persistenceEnabled, exists, err := unstructured.NestedBool(valuesObj, "persistence", "enabled")
	if !exists {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get expected persistence enabled: %w", err)
	}
	if !persistenceEnabled {
		return "", nil
	}

	// Get the expected persistence size from the values.
	expectedPersistenceSize, exists, err := unstructured.NestedString(valuesObj, "persistence", "size")
	if !exists {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get expected disk size: %w", err)
	}

	return expectedPersistenceSize, nil
}
