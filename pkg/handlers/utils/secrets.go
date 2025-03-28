// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
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

	return CreateSecretOnRemoteCluster(ctx, cl, credentialsOnRemote, cluster)

}

func CreateSecretOnRemoteCluster(
	ctx context.Context,
	cl ctrlclient.Client,
	credentialsOnRemote *corev1.Secret,
	cluster *clusterv1.Cluster,
) error {

	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	remoteClient, err := remote.NewClusterClient(ctx, "", cl, clusterKey)
	if err != nil {
		return fmt.Errorf("error creating client for remote cluster: %w", err)
	}

	err = EnsureNamespaceWithName(ctx, remoteClient, credentialsOnRemote.Namespace)
	if err != nil {
		return fmt.Errorf("error creating namespace on the remote cluster: %w", err)
	}

	err = client.ServerSideApply(ctx, remoteClient, credentialsOnRemote, client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("error creating Secret on the remote cluster: %w", err)
	}

	return nil
}

// EnsureClusterOwnerReferenceForObject ensures that OwnerReference of the cluster is added on provided object.
func EnsureClusterOwnerReferenceForObject(
	ctx context.Context,
	cl ctrlclient.Client,
	objectRef corev1.TypedLocalObjectReference,
	cluster *clusterv1.Cluster,
) error {
	targetObj, err := GetResourceFromTypedLocalObjectReference(
		ctx,
		cl,
		objectRef,
		cluster.Namespace,
	)
	if err != nil {
		return err
	}

	err = controllerutil.SetOwnerReference(cluster, targetObj, cl.Scheme())
	if err != nil {
		return fmt.Errorf("failed to set cluster's owner reference on object: %w", err)
	}

	err = cl.Update(ctx, targetObj)
	if err != nil {
		return fmt.Errorf("failed to update object with cluster's owner reference: %w", err)
	}
	return nil
}

// EnsureClusterOwnerReferenceForObject ensures that OwnerReference of the cluster is added on provided object.
func EnsureTypedObjectOwnerReference(
	ctx context.Context,
	cl ctrlclient.Client,
	targetObj corev1.TypedLocalObjectReference,
	ownerObj metav1.Object) error {
	resourceObj, err := GetResourceFromTypedLocalObjectReference(
		ctx,
		cl,
		targetObj,
		ownerObj.GetNamespace(),
	)
	if err != nil {
		return err
	}

	err = controllerutil.SetOwnerReference(
		ownerObj,
		resourceObj,
		cl.Scheme(),
		controllerutil.WithBlockOwnerDeletion(true)) // block deletion of owner object
	if err != nil {
		return fmt.Errorf("failed to set owner reference on object %s: %w", ownerObj.GetName(), err)
	}

	err = cl.Update(ctx, resourceObj)
	if err != nil {
		return fmt.Errorf(
			"failed to update owner reference on object %s: %w",
			resourceObj.GetName(),
			err,
		)
	}
	return nil
}

// GetResourceFromTypedLocalObjectReference gets the resource from the provided TypedLocalObjectReference.
func GetResourceFromTypedLocalObjectReference(
	ctx context.Context,
	cl ctrlclient.Client,
	typedLocalObjectRef corev1.TypedLocalObjectReference,
	ns string,
) (*unstructured.Unstructured, error) {
	apiVersion := corev1.SchemeGroupVersion.String()
	if typedLocalObjectRef.APIGroup != nil {
		apiVersion = *typedLocalObjectRef.APIGroup
	}

	objectRef := &corev1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       typedLocalObjectRef.Kind,
		Name:       typedLocalObjectRef.Name,
		Namespace:  ns,
	}

	targetObj, err := external.Get(ctx, cl, objectRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource from object reference: %w", err)
	}

	return targetObj, nil
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
