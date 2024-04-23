// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
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

var _ = Describe("Namespace", func() {
	It("creates a new namespace", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClient()
		Expect(err).To(BeNil())

		namespaceName := "new"

		Expect(EnsureNamespace(ctx, c, namespaceName)).To(Succeed())
		Expect(c.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		})).To((Succeed()))
	})

	It("updates a namespace, preserving user-managed fields", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClient()
		Expect(err).To(BeNil())

		namespaceName := "existing"
		Expect(c.Create(ctx,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Labels: map[string]string{
						"userkey": "uservalue",
					},
				},
			})).To(Succeed())

		Expect(EnsureNamespace(ctx, c, namespaceName)).To(Succeed())

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
		Expect(c.Get(ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
		Expect(ns.GetLabels()).To(HaveKeyWithValue("userkey", "uservalue"))

		Expect(c.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		})).To((Succeed()))
	})
})
