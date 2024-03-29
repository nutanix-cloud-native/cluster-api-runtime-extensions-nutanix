// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestCreateStorageClass(t *testing.T) {
	tests := []struct {
		name                 string
		defaultsNamespace    string
		storageConfig        v1alpha1.StorageClassConfig
		expectedStorageClass *storagev1.StorageClass
		provisioner          v1alpha1.StorageProvisioner
		isDefault            bool
	}{
		{
			name: "defaulting with AWS",
			storageConfig: v1alpha1.StorageClassConfig{
				Name:              "aws-ebs",
				ReclaimPolicy:     v1alpha1.VolumeReclaimDelete,
				VolumeBindingMode: v1alpha1.VolumeBindingWaitForFirstConsumer,
				Parameters:        nil,
			},
			expectedStorageClass: &storagev1.StorageClass{
				TypeMeta: metav1.TypeMeta{
					Kind:       kindStorageClass,
					APIVersion: storagev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-ebs",
					Namespace: "default",
				},
				Parameters:        defaultAWSStorageClassParams,
				ReclaimPolicy:     ptr.To(corev1.PersistentVolumeReclaimDelete),
				VolumeBindingMode: ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
				Provisioner:       string(v1alpha1.AWSEBSProvisioner),
			},
			provisioner:       v1alpha1.AWSEBSProvisioner,
			defaultsNamespace: "default",
		},
		{
			name: "nutanix for nutanix files",
			storageConfig: v1alpha1.StorageClassConfig{
				Name:              "nutanix-volumes",
				ReclaimPolicy:     v1alpha1.VolumeReclaimDelete,
				VolumeBindingMode: v1alpha1.VolumeBindingWaitForFirstConsumer,
				Parameters: map[string]string{
					"csi.storage.k8s.io/fstype": "ext4",
					"flashMode":                 "ENABLED",
					"storageContainer":          "storage-container-name",
					"chapAuth":                  "ENABLED",
					"storageType":               "NutanixVolumes",
					"whitelistIPMode":           "ENABLED",
					"whitelistIPAddr":           "1.1.1.1",
				},
			},
			expectedStorageClass: &storagev1.StorageClass{
				TypeMeta: metav1.TypeMeta{
					Kind:       kindStorageClass,
					APIVersion: storagev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nutanix-volumes",
					Namespace: "default",
				},
				Parameters: map[string]string{
					"csi.storage.k8s.io/fstype": "ext4",
					"flashMode":                 "ENABLED",
					"storageContainer":          "storage-container-name",
					"chapAuth":                  "ENABLED",
					"storageType":               "NutanixVolumes",
					"whitelistIPMode":           "ENABLED",
					"whitelistIPAddr":           "1.1.1.1",
				},
				ReclaimPolicy:     ptr.To(corev1.PersistentVolumeReclaimDelete),
				VolumeBindingMode: ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
				Provisioner:       string(v1alpha1.NutanixProvisioner),
			},
			provisioner:       v1alpha1.NutanixProvisioner,
			defaultsNamespace: "default",
		},
		{
			name: "nutanix defaults",
			storageConfig: v1alpha1.StorageClassConfig{
				Name:              "nutanix-volumes",
				ReclaimPolicy:     v1alpha1.VolumeReclaimDelete,
				VolumeBindingMode: v1alpha1.VolumeBindingWaitForFirstConsumer,
			},
			expectedStorageClass: &storagev1.StorageClass{
				TypeMeta: metav1.TypeMeta{
					Kind:       kindStorageClass,
					APIVersion: storagev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nutanix-volumes",
					Namespace: "default",
				},
				ReclaimPolicy:     ptr.To(corev1.PersistentVolumeReclaimDelete),
				VolumeBindingMode: ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
				Provisioner:       string(v1alpha1.NutanixProvisioner),
			},
			provisioner:       v1alpha1.NutanixProvisioner,
			defaultsNamespace: "default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := CreateStorageClass(
				tt.storageConfig,
				tt.defaultsNamespace,
				tt.provisioner,
				false,
			)
			if diff := cmp.Diff(sc, tt.expectedStorageClass); diff != "" {
				t.Errorf("CreateStorageClass() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateConfigMapForCRS(t *testing.T) {
	tests := []struct {
		name          string
		testCMName    string
		testNamespace string
		objs          []runtime.Object
		expectedCM    corev1.ConfigMap
	}{
		{
			name:          "multiple storage class objects",
			testCMName:    "test",
			testNamespace: "default",
			objs: []runtime.Object{
				&storagev1.StorageClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       kindStorageClass,
						APIVersion: storagev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "default",
					},
				},
				&storagev1.StorageClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       kindStorageClass,
						APIVersion: storagev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-2",
						Namespace: "default",
					},
				},
			},
			expectedCM: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "ConfigMap",
				},
				Data: map[string]string{
					defaultCRSConfigMapKey: `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  creationTimestamp: null
  name: test
  namespace: default
provisioner: ""
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  creationTimestamp: null
  name: test-2
  namespace: default
provisioner: ""`,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := CreateConfigMapForCRS(tt.testCMName, tt.testNamespace, tt.objs...)
			if err != nil {
				t.Errorf("failed to create cm with error %v", err)
			}
			data, ok := cm.Data[defaultCRSConfigMapKey]
			if !ok {
				t.Errorf("expected %s to exist in cm.Data. got %v", defaultCRSConfigMapKey, cm.Data)
			}
			expected := tt.expectedCM.Data[defaultCRSConfigMapKey]
			if data != expected {
				t.Errorf("expected %s \n got %s", expected, data)
			}
		})
	}
}
