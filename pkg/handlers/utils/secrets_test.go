// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
	}
	testSecretWithOwnerRef = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       testCluster.Name,
					UID:        "12345",
				},
			},
		},
	}

	testCluster = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
	}
	anotherTestCluster = &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "another-test-cluster",
		},
	}
)

func Test_EnsureOwnerRefForSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		client        client.Client
		secretName    string
		cluster       *clusterv1.Cluster
		wantOwnerRefs int
		wantErr       error
	}{
		{
			name:          "add owner reference to Secret",
			client:        buildFakeClient(t, testSecret, testCluster),
			secretName:    testSecret.Name,
			cluster:       testCluster,
			wantOwnerRefs: 1,
		},
		{
			name:          "update existing owner reference in Secret",
			client:        buildFakeClient(t, testSecretWithOwnerRef, testCluster),
			secretName:    testSecretWithOwnerRef.Name,
			cluster:       testCluster,
			wantOwnerRefs: 1,
		},
		{
			name:          "add owner reference to Secret with an existing owner reference",
			client:        buildFakeClient(t, testSecretWithOwnerRef, anotherTestCluster),
			secretName:    testSecretWithOwnerRef.Name,
			cluster:       anotherTestCluster,
			wantOwnerRefs: 2,
		},
		{
			name:       "should error on a missing Secret",
			client:     buildFakeClient(t, testSecret, testCluster),
			secretName: "missing-secret",
			cluster:    testCluster,
			wantErr:    errors.NewNotFound(corev1.Resource("secrets"), "missing-secret"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := EnsureOwnerReferenceForSecret(
				context.Background(),
				tt.client,
				tt.secretName,
				tt.cluster,
			)
			require.Equal(t, tt.wantErr, err)
			if tt.wantErr != nil {
				return
			}
			// verify that the owner reference was added
			secret := &corev1.Secret{}
			err = tt.client.Get(
				context.Background(),
				client.ObjectKey{Namespace: tt.cluster.Namespace, Name: tt.secretName},
				secret,
			)
			require.NoError(t, err, "failed to get updated secret")
			assert.Len(t, secret.OwnerReferences, tt.wantOwnerRefs)
		})
	}
}

func buildFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))
	return fake.NewClientBuilder().WithScheme(clientScheme).WithObjects(objs...).Build()
}
