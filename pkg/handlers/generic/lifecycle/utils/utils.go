// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

const (
	kindStorageClass      = "StorageClass"
	awsEBSProvisionerName = "ebs.csi.aws.com"
)

var (
	defualtStorageClassKey = "storageclass.kubernetes.io/is-default-class"
	defaultStorageClassMap = map[string]string{
		defualtStorageClassKey: "true",
	}
	defaultParams = map[string]string{
		"csi.storage.k8s.io/fstype": "ext4",
		"type":                      "gp3",
	}
)

func EnsureCRSForClusterFromConfigMaps(
	ctx context.Context,
	crsName string,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	configMaps ...*corev1.ConfigMap,
) error {
	resources := make([]crsv1.ResourceRef, 0, len(configMaps))
	for _, cm := range configMaps {
		resources = append(resources, crsv1.ResourceRef{
			Kind: string(crsv1.ConfigMapClusterResourceSetResourceKind),
			Name: cm.Name,
		})
	}

	crs := &crsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: crsv1.GroupVersion.String(),
			Kind:       "ClusterResourceSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      crsName,
		},
		Spec: crsv1.ClusterResourceSetSpec{
			Resources: resources,
			Strategy:  string(crsv1.ClusterResourceSetStrategyReconcile),
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(cluster, crs, c.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	err := client.ServerSideApply(ctx, c, crs)
	if err != nil {
		return fmt.Errorf("failed to server side apply %w", err)
	}

	return nil
}

// EnsureNamespace will create the namespece if it does not exist.
func EnsureNamespace(ctx context.Context, c ctrlclient.Client, name string) error {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	// check if namespace exists and return early if it does
	if err := c.Get(ctx, ctrlclient.ObjectKeyFromObject(ns), ns); err == nil {
		return nil
	}

	err := client.ServerSideApply(ctx, c, ns)
	if err != nil {
		return fmt.Errorf("failed to server side apply %w", err)
	}

	return nil
}

func RetrieveValuesTemplateConfigMap(
	ctx context.Context,
	c ctrlclient.Client,
	configMapName,
	defaultsNamespace string,
) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultsNamespace,
			Name:      configMapName,
		},
	}
	configMapObjName := ctrlclient.ObjectKeyFromObject(
		configMap,
	)
	err := c.Get(ctx, configMapObjName, configMap)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to retrieve installation values template ConfigMap %q: %w",
			configMapObjName,
			err,
		)
	}
	return configMap, nil
}

func CreateStorageClass(
	ctx context.Context,
	cl ctrlclient.Client,
	storageConfig v1alpha1.StorageClassConfig,
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	isDefault bool,
) error {
	var volumeBindingMode *storagev1.VolumeBindingMode
	switch storageConfig.VolumeBindingMode {
	case v1alpha1.VolumeBindingImmediate:
		volumeBindingMode = ptr.To(storagev1.VolumeBindingImmediate)
	case v1alpha1.VolumeBindingWaitForFirstConsumer:
	default:
		volumeBindingMode = ptr.To(storagev1.VolumeBindingWaitForFirstConsumer)
	}
	var reclaimPolicy *corev1.PersistentVolumeReclaimPolicy
	switch storageConfig.ReclaimPolicy {
	case v1alpha1.VolumeReclaimRecycle:
		reclaimPolicy = ptr.To(corev1.PersistentVolumeReclaimRecycle)
	case v1alpha1.VolumeReclaimDelete:
		reclaimPolicy = ptr.To(corev1.PersistentVolumeReclaimDelete)
	case v1alpha1.VolumeReclaimRetain:
		reclaimPolicy = ptr.To(corev1.PersistentVolumeReclaimRetain)
	}
	params := defaultParams
	if storageConfig.Parameters != nil {
		params = maps.Clone(storageConfig.Parameters)
	}
	sc := storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: storagev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      storageConfig.Name,
			Namespace: defaultsNamespace,
		},
		Provisioner:       awsEBSProvisionerName,
		Parameters:        params,
		VolumeBindingMode: volumeBindingMode,
		ReclaimPolicy:     reclaimPolicy,
	}
	if isDefault {
		sc.ObjectMeta.Annotations = defaultStorageClassMap
	}
	workloadClient, err := remote.NewClusterClient(
		ctx,
		"",
		cl,
		capiutil.ObjectKey(cluster),
	)
	if err != nil {
		return err
	}

	if err := client.ServerSideApply(ctx, workloadClient, &sc); err != nil {
		return fmt.Errorf(
			"failed to create storage class %w",
			err,
		)
	}
	return nil
}
