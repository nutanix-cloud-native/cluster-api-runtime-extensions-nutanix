// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_GetFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		controlPlaneEndpointSpec v1alpha1.ControlPlaneEndpointSpec
		configMap                *corev1.ConfigMap
		expectedContent          string
		expectedErr              error
	}{
		{
			name: "should return templated data with both host and port",
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "10.20.100.10",
				Port: 6443,
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-kube-vip-template",
					Namespace: "default",
				},
				Data: map[string]string{
					"data": validKubeVIPTemplate,
				},
			},
			expectedContent: expectedKubeVIPPod,
		},
	}

	for idx := range tests {
		tt := tests[idx] // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fakeClient := fake.NewClientBuilder().WithObjects(tt.configMap).Build()

			provider := kubeVIPFromConfigMapProvider{
				client:       fakeClient,
				configMapKey: client.ObjectKeyFromObject(tt.configMap),
			}

			file, err := provider.GetFile(context.TODO(), tt.controlPlaneEndpointSpec)
			require.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedContent, file.Content)
			assert.NotEmpty(t, file.Path)
			assert.NotEmpty(t, file.Owner)
			assert.NotEmpty(t, file.Permissions)
		})
	}
}

func Test_getTemplateFromConfigMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		configMap    *corev1.ConfigMap
		expectedData string
		expectedErr  error
	}{
		{
			name: "should return data from the only key",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-kube-vip-template",
					Namespace: "default",
				},
				Data: map[string]string{
					"data": "kube-vip-template",
				},
			},
			expectedData: "kube-vip-template",
		},
		{
			name: "should fail with multipleKeysError",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-kube-vip-template",
					Namespace: "default",
				},
				Data: map[string]string{
					"data":           "kube-vip-template",
					"unexpected-key": "unexpected-value",
				},
			},
			expectedErr: multipleKeysError{
				configMapKey: client.ObjectKey{
					Name:      "default-kube-vip-template",
					Namespace: "default",
				},
			},
		},
		{
			name: "should fail with emptyValuesError",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-kube-vip-template",
					Namespace: "default",
				},
				Data: map[string]string{
					"data": "",
				},
			},
			expectedErr: emptyValuesError{
				configMapKey: client.ObjectKey{
					Name:      "default-kube-vip-template",
					Namespace: "default",
				},
			},
		},
	}

	for idx := range tests {
		tt := tests[idx] // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fakeClient := fake.NewClientBuilder().WithObjects(tt.configMap).Build()

			data, err := getTemplateFromConfigMap(
				context.TODO(),
				fakeClient,
				client.ObjectKeyFromObject(tt.configMap),
			)
			require.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedData, data)
		})
	}
}

var (
	validKubeVIPTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: kube-vip
  namespace: kube-system
spec:
  containers:
    - name: kube-vip
      image: ghcr.io/kube-vip/kube-vip:v1.1.1
      imagePullPolicy: IfNotPresent
      args:
        - manager
      env:
        - name: vip_arp
          value: "true"
        - name: address
          value: "{{ .ControlPlaneEndpoint.Host }}"
        - name: port
          value: "{{ .ControlPlaneEndpoint.Port }}"
`

	expectedKubeVIPPod = `
apiVersion: v1
kind: Pod
metadata:
  name: kube-vip
  namespace: kube-system
spec:
  containers:
    - name: kube-vip
      image: ghcr.io/kube-vip/kube-vip:v1.1.1
      imagePullPolicy: IfNotPresent
      args:
        - manager
      env:
        - name: vip_arp
          value: "true"
        - name: address
          value: "10.20.100.10"
        - name: port
          value: "6443"
`
)
