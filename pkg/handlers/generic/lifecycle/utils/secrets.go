// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
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
	sourceSecret, err := getSecretForCluster(ctx, cl, srcSecretName, cluster)
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

	err = EnsureNamespaceWithName(ctx, remoteClient, dstSecretKey.Namespace)
	if err != nil {
		return fmt.Errorf("error creating namespace on the remote cluster: %w", err)
	}

	err = client.ServerSideApply(ctx, remoteClient, credentialsOnRemote, client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("error creating Secret on the remote cluster: %w", err)
	}

	return nil
}

// EnsureOwnerRefForSecret will ensure that the secretName Secret has an OwnerReference of the cluster.
func EnsureOwnerRefForSecret(
	ctx context.Context,
	cl ctrlclient.Client,
	secretName string,
	cluster *clusterv1.Cluster,
) error {
	secret, err := getSecretForCluster(ctx, cl, secretName, cluster)
	if err != nil {
		return err
	}

	secret.OwnerReferences = util.EnsureOwnerRef(
		secret.OwnerReferences,
		metav1.OwnerReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       cluster.Kind,
			UID:        cluster.UID,
			Name:       cluster.Name,
		},
	)

	err = cl.Update(ctx, secret)
	if err != nil {
		return fmt.Errorf("failed to update Secret with owner references: %w", err)
	}
	return nil
}

func getSecretForCluster(
	ctx context.Context,
	c ctrlclient.Client,
	secretName string,
	cluster *clusterv1.Cluster,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cluster.Namespace,
		},
	}
	return secret, c.Get(ctx, ctrlclient.ObjectKeyFromObject(secret), secret)
}
