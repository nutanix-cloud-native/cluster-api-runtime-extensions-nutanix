// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"
	"maps"

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
	KindStorageClass = "StorageClass"

	// isDefaultStorageClassAnnotation represents a StorageClass annotation that
	// marks a class as the default StorageClass.
	isDefaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"
)

var defaultStorageClassMap = map[string]string{
	isDefaultStorageClassAnnotation: "true",
}

func CreateStorageClass(
	providerName string,
	storageClassName string,
	storageClassConfig v1alpha1.StorageClassConfig,
	provisioner v1alpha1.StorageProvisioner,
	isDefault bool,
	defaultParameters map[string]string,
) *storagev1.StorageClass {
	parameters := make(map[string]string, len(defaultParameters)+len(storageClassConfig.Parameters))
	// set the defaults first so that user provided parameters can override them
	maps.Copy(parameters, defaultParameters)
	// set user provided parameters, overriding any defaults with the same key
	maps.Copy(parameters, storageClassConfig.Parameters)

	sc := storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindStorageClass,
			APIVersion: storagev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: providerName + "-" + storageClassName,
		},
		Provisioner:          string(provisioner),
		Parameters:           parameters,
		VolumeBindingMode:    storageClassConfig.VolumeBindingMode,
		ReclaimPolicy:        storageClassConfig.ReclaimPolicy,
		AllowVolumeExpansion: ptr.To(storageClassConfig.AllowExpansion),
	}
	if isDefault {
		sc.ObjectMeta.Annotations = defaultStorageClassMap
	}
	return &sc
}

func CreateStorageClassesOnRemote(
	ctx context.Context,
	cl ctrlclient.Client,
	configs map[string]v1alpha1.StorageClassConfig,
	cluster *clusterv1.Cluster,
	defaultStorage v1alpha1.DefaultStorage,
	csiProvider string,
	provisioner v1alpha1.StorageProvisioner,
	defaultParameters map[string]string,
) error {
	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		cl,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating client for remote cluster: %w", err)
	}

	for name, config := range configs {
		setAsDefault := csiProvider == defaultStorage.Provider &&
			name == defaultStorage.StorageClassConfig
		sc := CreateStorageClass(
			csiProvider,
			name,
			config,
			provisioner,
			setAsDefault,
			defaultParameters,
		)
		if err := client.ServerSideApply(ctx, remoteClient, sc, client.ForceOwnership); err != nil {
			return fmt.Errorf("error creating storage class %v on remote cluster: %w", sc, err)
		}
	}

	return nil
}
