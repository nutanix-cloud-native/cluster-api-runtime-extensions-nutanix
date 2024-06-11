// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func copyClusterClassAndTemplates(
	ctx context.Context,
	w client.Writer,
	templateReader client.Reader,
	source *clusterv1.ClusterClass,
	namespace string,
) error {
	target := source.DeepCopy()
	target.SetNamespace(namespace)
	target.SetResourceVersion("")

	if err := walkReferences(ctx, target, func(ctx context.Context, ref *corev1.ObjectReference) error {
		// Get referenced Template
		sourceTemplate, err := getReference(ctx, templateReader, ref)
		if err != nil {
			return fmt.Errorf("failed to get reference: %w", err)
		}

		// Copy Template to target namespace
		targetTemplate := sourceTemplate.DeepCopy()
		targetTemplate.SetNamespace(namespace)
		targetTemplate.SetResourceVersion("")

		if err := w.Create(ctx, targetTemplate); err != nil {
			return fmt.Errorf(
				"failed to create %s %s: %w",
				targetTemplate.GetKind(),
				client.ObjectKeyFromObject(targetTemplate),
				err,
			)
		}

		// Update reference to point to newly created Template
		ref.UID = targetTemplate.GetUID()
		ref.Namespace = targetTemplate.GetNamespace()

		return nil
	}); err != nil {
		return fmt.Errorf("error processing references: %w", err)
	}

	if err := w.Create(ctx, target); err != nil {
		return fmt.Errorf(
			"failed to create %s %s: %w",
			target.Kind,
			client.ObjectKeyFromObject(target),
			err,
		)
	}
	return nil
}
