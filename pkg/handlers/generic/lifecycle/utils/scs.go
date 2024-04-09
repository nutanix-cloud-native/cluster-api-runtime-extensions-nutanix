// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
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
		VolumeBindingMode:    ptr.To(storageConfig.VolumeBindingMode),
		ReclaimPolicy:        ptr.To(storageConfig.ReclaimPolicy),
		AllowVolumeExpansion: ptr.To(storageConfig.AllowExpansion),
	}
	if isDefault {
		sc.ObjectMeta.Annotations = defaultStorageClassMap
	}
	return &sc
}
