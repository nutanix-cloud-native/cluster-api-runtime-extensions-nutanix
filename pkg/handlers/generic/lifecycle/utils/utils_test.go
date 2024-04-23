// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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
