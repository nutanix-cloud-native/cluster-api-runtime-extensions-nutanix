// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

const (
	RegistryAddonRootCASecretName = "registry-addon-root-ca"
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

	secretOnRemote := &corev1.Secret{
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

	err = client.ServerSideApply(ctx, remoteClient, secretOnRemote, client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("error creating Secret on the remote cluster: %w", err)
	}

	return nil
}

// EnsureSecretOnRemoteCluster ensures that the given Secret exists on the remote cluster.
func EnsureSecretOnRemoteCluster(
	ctx context.Context,
	cl ctrlclient.Client,
	secret *corev1.Secret,
	cluster *clusterv1.Cluster,
) error {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	remoteClient, err := remote.NewClusterClient(ctx, "", cl, clusterKey)
	if err != nil {
		return fmt.Errorf("error creating client for remote cluster: %w", err)
	}

	err = EnsureNamespaceWithName(ctx, remoteClient, secret.Namespace)
	if err != nil {
		return fmt.Errorf("error creating namespace on the remote cluster: %w", err)
	}

	err = client.ServerSideApply(ctx, remoteClient, secret, client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("error creating Secret on the remote cluster: %w", err)
	}

	return nil
}

func EnsureSecretForLocalCluster(
	ctx context.Context,
	cl ctrlclient.Client,
	secret *corev1.Secret,
	cluster *clusterv1.Cluster,
) error {
	if secret.Namespace != "" &&
		secret.Namespace != cluster.Namespace {
		return fmt.Errorf(
			"secret namespace %q does not match cluster namespace %q",
			secret.Namespace,
			cluster.Namespace,
		)
	}

	err := controllerutil.SetOwnerReference(cluster, secret, cl.Scheme())
	if err != nil {
		return fmt.Errorf("failed to set cluster's owner reference on Secret: %w", err)
	}

	err = client.ServerSideApply(ctx, cl, secret, client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("error creating Secret for cluster: %w", err)
	}

	return nil
}

// SecretForImageRegistryCredentials returns the Secret for the given ImageRegistryCredentials.
// Returns nil if the secret field is empty.
func SecretForImageRegistryCredentials(
	ctx context.Context,
	c ctrlclient.Reader,
	credentials *v1alpha1.RegistryCredentials,
	objectNamespace string,
) (*corev1.Secret, error) {
	name := SecretNameForImageRegistryCredentials(credentials)
	if name == "" {
		return nil, nil
	}

	key := ctrlclient.ObjectKey{
		Name:      name,
		Namespace: objectNamespace,
	}
	secret := &corev1.Secret{}
	err := c.Get(ctx, key, secret)
	return secret, err
}

// SecretNameForImageRegistryCredentials returns the name of the Secret for the given RegistryCredentials.
// Returns an empty string if the credentials or secret field is empty.
func SecretNameForImageRegistryCredentials(credentials *v1alpha1.RegistryCredentials) string {
	if credentials == nil || credentials.SecretRef == nil || credentials.SecretRef.Name == "" {
		return ""
	}
	return credentials.SecretRef.Name
}

func SecretForRegistryAddonRootCA(
	ctx context.Context,
	c ctrlclient.Reader,
) (*corev1.Secret, error) {
	secret, err := findSecret(ctx, c, RegistryAddonRootCASecretName)
	if err != nil {
		return nil, fmt.Errorf("error getting registry addon root CA secret: %w", err)
	}
	return secret, nil
}

func SecretForClusterRegistryAddonCA(
	ctx context.Context,
	c ctrlclient.Reader,
	cluster *clusterv1.Cluster,
) (*corev1.Secret, error) {
	secret, err := getSecretForCluster(ctx, c, SecretNameForRegistryAddonCA(cluster), cluster)
	if err != nil {
		return nil, fmt.Errorf("error getting registry addon CA secret for cluster: %w", err)
	}
	return secret, nil
}

// SecretNameForRegistryAddonCA returns the name of the registry addon CA Secret.
func SecretNameForRegistryAddonCA(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("%s-registry-addon-ca", cluster.Name)
}

func getSecretForCluster(
	ctx context.Context,
	c ctrlclient.Reader,
	secretName string,
	cluster *clusterv1.Cluster,
) (*corev1.Secret, error) {
	return getSecret(ctx, c, secretName, cluster.Namespace)
}

func getSecret(
	ctx context.Context,
	c ctrlclient.Reader,
	secretName string,
	secretNamespace string,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}
	return secret, c.Get(ctx, ctrlclient.ObjectKeyFromObject(secret), secret)
}

type SecretNotFoundError struct {
	name string
}

func (e *SecretNotFoundError) Error() string {
	return fmt.Sprintf("secrets %q not found", e.name)
}

func IsSecretNotFoundError(err error) bool {
	var e *SecretNotFoundError
	return errors.As(err, &e)
}

type MultipleSecretsFoundError struct {
	name string
}

func (e *MultipleSecretsFoundError) Error() string {
	return fmt.Sprintf("multiple Secrets found with name %q", e.name)
}

// findSecret finds a Secret by name across all namespaces.
// It returns an error if multiple Secrets with the same name are found.
// Returns nil if no Secret is found.
func findSecret(
	ctx context.Context,
	c ctrlclient.Reader,
	secretName string,
) (*corev1.Secret, error) {
	secrets := &corev1.SecretList{}
	err := c.List(ctx, secrets)
	if err != nil {
		return nil, fmt.Errorf("error listing secrets: %w", err)
	}

	matches := make([]corev1.Secret, 0)
	for i := range secrets.Items {
		secret := secrets.Items[i]
		if secret.Name == secretName {
			matches = append(matches, secret)
		}
	}
	switch len(matches) {
	case 1:
		return &matches[0], nil
	case 0:
		return nil, &SecretNotFoundError{name: secretName}
	default:
		return nil, &MultipleSecretsFoundError{name: secretName}
	}
}
