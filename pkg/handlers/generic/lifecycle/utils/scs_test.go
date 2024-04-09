// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

var (
	defaultParameters = map[string]string{
		"csi.storage.k8s.io/fstype": "ext4",
		"type":                      "gp3",
	}
	userProviderParameters = map[string]string{
		"csi.storage.k8s.io/fstype": "xfs",
		"flashMode":                 "ENABLED",
		"storageContainer":          "storage-container-name",
		"chapAuth":                  "ENABLED",
		"storageType":               "NutanixVolumes",
		"whitelistIPMode":           "ENABLED",
		"whitelistIPAddr":           "1.1.1.1",
	}

	combinedParameters = map[string]string{
		"csi.storage.k8s.io/fstype": "xfs",
		"type":                      "gp3",
		"flashMode":                 "ENABLED",
		"storageContainer":          "storage-container-name",
		"chapAuth":                  "ENABLED",
		"storageType":               "NutanixVolumes",
		"whitelistIPMode":           "ENABLED",
		"whitelistIPAddr":           "1.1.1.1",
	}
)

func TestCreateStorageClass(t *testing.T) {
	tests := []struct {
		name                 string
		storageConfig        v1alpha1.StorageClassConfig
		provisioner          v1alpha1.StorageProvisioner
		setAsDefault         bool
		defaultParameters    map[string]string
		expectedStorageClass *storagev1.StorageClass
	}{
		{
			name: "with only default parameters",
			storageConfig: v1alpha1.StorageClassConfig{
				Name:              "aws-ebs",
				ReclaimPolicy:     v1alpha1.VolumeReclaimDelete,
				VolumeBindingMode: v1alpha1.VolumeBindingWaitForFirstConsumer,
				Parameters:        nil,
				AllowExpansion:    true,
			},
			provisioner:       v1alpha1.AWSEBSProvisioner,
			defaultParameters: defaultParameters,
			expectedStorageClass: &storagev1.StorageClass{
				TypeMeta: metav1.TypeMeta{
					Kind:       kindStorageClass,
					APIVersion: storagev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "aws-ebs",
				},
				Parameters:           defaultParameters,
				ReclaimPolicy:        ptr.To(corev1.PersistentVolumeReclaimDelete),
				VolumeBindingMode:    ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
				Provisioner:          string(v1alpha1.AWSEBSProvisioner),
				AllowVolumeExpansion: ptr.To(true),
			},
		},
		{
			name: "with only user provided parameters",
			storageConfig: v1alpha1.StorageClassConfig{
				Name:              "nutanix-volumes",
				ReclaimPolicy:     v1alpha1.VolumeReclaimDelete,
				VolumeBindingMode: v1alpha1.VolumeBindingWaitForFirstConsumer,
				Parameters:        userProviderParameters,
			},
			provisioner: v1alpha1.NutanixProvisioner,
			expectedStorageClass: &storagev1.StorageClass{
				TypeMeta: metav1.TypeMeta{
					Kind:       kindStorageClass,
					APIVersion: storagev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "nutanix-volumes",
				},
				Parameters:           userProviderParameters,
				ReclaimPolicy:        ptr.To(corev1.PersistentVolumeReclaimDelete),
				VolumeBindingMode:    ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
				Provisioner:          string(v1alpha1.NutanixProvisioner),
				AllowVolumeExpansion: ptr.To(false),
			},
		},
		{
			name: "with both default and user provided parameters",
			storageConfig: v1alpha1.StorageClassConfig{
				Name:              "aws-ebs",
				ReclaimPolicy:     v1alpha1.VolumeReclaimDelete,
				VolumeBindingMode: v1alpha1.VolumeBindingWaitForFirstConsumer,
				Parameters:        userProviderParameters,
				AllowExpansion:    true,
			},
			provisioner:       v1alpha1.AWSEBSProvisioner,
			defaultParameters: defaultParameters,
			expectedStorageClass: &storagev1.StorageClass{
				TypeMeta: metav1.TypeMeta{
					Kind:       kindStorageClass,
					APIVersion: storagev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "aws-ebs",
				},
				Parameters:           combinedParameters,
				ReclaimPolicy:        ptr.To(corev1.PersistentVolumeReclaimDelete),
				VolumeBindingMode:    ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
				Provisioner:          string(v1alpha1.AWSEBSProvisioner),
				AllowVolumeExpansion: ptr.To(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := CreateStorageClass(
				tt.storageConfig,
				tt.provisioner,
				tt.setAsDefault,
				tt.defaultParameters,
			)
			if diff := cmp.Diff(sc, tt.expectedStorageClass); diff != "" {
				t.Errorf("CreateStorageClass() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
