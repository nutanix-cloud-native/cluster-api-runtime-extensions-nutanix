// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

const (
	kindStorageClass = "StorageClass"
)

func CreateStorageClass(
	storageConfig v1alpha1.StorageClassConfig,
	provisionerName v1alpha1.StorageProvisioner,
	isDefault bool,
	defaultParameters map[string]string,
) *storagev1.StorageClass {
	parameters := make(map[string]string)
	// set the defaults first so that user provided parameters can override them
	for k, v := range defaultParameters {
		parameters[k] = v
	}
	// set user provided parameters, overriding any defaults with the same key
	for k, v := range storageConfig.Parameters {
		parameters[k] = v
	}

	sc := storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       kindStorageClass,
			APIVersion: storagev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: storageConfig.Name,
		},
		Provisioner:          string(provisionerName),
		Parameters:           parameters,
		VolumeBindingMode:    storageConfig.VolumeBindingMode,
		ReclaimPolicy:        storageConfig.ReclaimPolicy,
		AllowVolumeExpansion: ptr.To(storageConfig.AllowExpansion),
	}
	if isDefault {
		sc.ObjectMeta.Annotations = defaultStorageClassMap
	}
	return &sc
}

func CreateStorageClassOnRemote(
	ctx context.Context,
	cl ctrlclient.Client,
	configs []v1alpha1.StorageClassConfig,
	cluster *clusterv1.Cluster,
	defaultStorageConfig v1alpha1.DefaultStorage,
	csiProvider string,
	provisioner v1alpha1.StorageProvisioner,
	defaultParameters map[string]string,
) error {
	allStorageClasses := make([]*storagev1.StorageClass, 0, len(configs))
	for _, config := range configs {
		setAsDefault := config.Name == defaultStorageConfig.StorageClassConfigName &&
			csiProvider == defaultStorageConfig.ProviderName
		allStorageClasses = append(allStorageClasses, CreateStorageClass(
			config,
			provisioner,
			setAsDefault,
			defaultParameters,
		))
	}
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	remoteClient, err := remote.NewClusterClient(ctx, "", cl, clusterKey)
	if err != nil {
		return fmt.Errorf("error creating client for remote cluster: %w", err)
	}
	for _, sc := range allStorageClasses {
		err = client.ServerSideApply(ctx, remoteClient, sc, client.ForceOwnership)
		if err != nil {
			return fmt.Errorf("error creating storage class %v on remote cluster: %w", sc, err)
		}
	}
	return nil
}
