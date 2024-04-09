// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

// CopySecretToRemoteCluster will get the Secret from srcSecretName
// and create it on the remote cluster, copying Data and StringData to dstSecretKey Secret.
func CopySecretToRemoteCluster(
	ctx context.Context,
	cl ctrlclient.Client,
	srcSecretName string,
	dstSecretKey ctrlclient.ObjectKey,
	cluster *clusterv1.Cluster,
) error {
	sourceSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      srcSecretName,
			Namespace: cluster.Namespace,
		},
	}
	err := cl.Get(ctx, ctrlclient.ObjectKeyFromObject(sourceSecret), sourceSecret)
	if err != nil {
		return err
	}

	credentialsOnRemote := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dstSecretKey.Name,
			Namespace: dstSecretKey.Namespace,
		},
		Data:       sourceSecret.Data,
		StringData: sourceSecret.StringData,
	}

	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	remoteClient, err := remote.NewClusterClient(ctx, "", cl, clusterKey)
	if err != nil {
		return fmt.Errorf("error creating client for remote cluster: %w", err)
	}

	err = EnsureNamespace(ctx, remoteClient, dstSecretKey.Namespace)
	if err != nil {
		return fmt.Errorf("error creating namespace on the remote cluster: %w", err)
	}

	err = client.ServerSideApply(ctx, remoteClient, credentialsOnRemote)
	if err != nil {
		return fmt.Errorf("error creating Secret on the remote cluster: %w", err)
	}

	return nil
}
