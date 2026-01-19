// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// EnsureClusterOwnerReferenceForObject ensures that OwnerReference of the cluster is added on provided object.
func EnsureClusterOwnerReferenceForObject(
	ctx context.Context,
	cl ctrlclient.Client,
	objectRef corev1.TypedLocalObjectReference,
	cluster *clusterv1.Cluster,
) error {
	targetObj, err := getResourceFromTypedLocalObjectReference(
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

// getResourceFromTypedLocalObjectReference gets the resource from the provided TypedLocalObjectReference.
func getResourceFromTypedLocalObjectReference(
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
