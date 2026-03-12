// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func copyClusterClassAndTemplates(
	ctx context.Context,
	w client.Writer,
	templateReader client.Reader,
	scheme *runtime.Scheme,
	source *clusterv1beta2.ClusterClass,
	namespace string,
) error {
	target := copyObjectForCreate(source, source.Name, namespace)

	// Use source (not target) for walking references so we fetch templates from the source namespace.
	if err := walkReferences(ctx, source, func(ctx context.Context, ref *corev1.ObjectReference) error {
		// Get referenced Template
		sourceTemplate, err := getReference(ctx, templateReader, scheme, ref)
		if err != nil {
			return fmt.Errorf("failed to get reference: %w", err)
		}

		// Copy Template to target namespace, if it does not exist there.
		targetTemplate := copyObjectForCreate(sourceTemplate, sourceTemplate.GetName(), namespace)
		if err = createIfNotExists(ctx, templateReader, w, scheme, targetTemplate); err != nil {
			return fmt.Errorf("failed to create template: %w", err)
		}

		// Update reference to point to newly created Template
		ref.UID = targetTemplate.GetUID()
		ref.Namespace = targetTemplate.GetNamespace()

		return nil
	}); err != nil {
		return fmt.Errorf("error processing references: %w", err)
	}

	// Copy ClusterClass to target namespace, if it does not exist there.
	if err := createIfNotExists(ctx, templateReader, w, scheme, target); err != nil {
		return fmt.Errorf("failed to create cluster class: %w", err)
	}
	return nil
}

func createIfNotExists(
	ctx context.Context,
	r client.Reader,
	w client.Writer,
	scheme *runtime.Scheme,
	obj client.Object,
) error {
	key := client.ObjectKeyFromObject(obj)
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return errors.Wrapf(err, "failed to get GVK for object %s/%s", obj.GetNamespace(), obj.GetName())
	}

	// Check if the resource exists.
	// We do not need the object itself, just the metadata, so we use PartialObjectMetadata.
	partial := &metav1.PartialObjectMetadata{}
	partial.SetGroupVersionKind(gvk)
	err = r.Get(ctx, key, partial)

	if apierrors.IsNotFound(err) {
		// The resource does not exist, so create it.
		if err := w.Create(ctx, obj); err != nil {
			return fmt.Errorf("failed to create %s %s: %w", gvk, key, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check if %s %s exists: %w", gvk, key, err)
	}
	return nil
}

// copyObjectForCreate copies the object, updating the name and namespace,
// and preserving only labels and annotations metadata.
func copyObjectForCreate[T client.Object](src T, name, namespace string) T {
	dst := src.DeepCopyObject().(T)

	dst.SetName(name)
	dst.SetNamespace(namespace)

	// Zero out ManagedFields (clients will set them)
	dst.SetManagedFields(nil)
	// Zero out OwnerReferences (object is garbage-collected if
	// owners are not in the target namespace)
	dst.SetOwnerReferences(nil)

	// Zero out fields that are ignored by the API server on create
	dst.SetCreationTimestamp(metav1.Time{})
	dst.SetDeletionGracePeriodSeconds(nil)
	dst.SetDeletionTimestamp(nil)
	dst.SetFinalizers(nil)
	dst.SetGenerateName("")
	dst.SetGeneration(0)
	dst.SetLabels(nil)
	dst.SetManagedFields(nil)
	dst.SetOwnerReferences(nil)
	dst.SetResourceVersion("")
	dst.SetSelfLink("")
	dst.SetUID("")

	return dst
}
