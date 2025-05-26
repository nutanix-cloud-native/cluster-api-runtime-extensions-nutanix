// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespacesync

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getReference gets the object referenced in ref.
func getReference(
	ctx context.Context,
	cli client.Reader,
	ref *corev1.ObjectReference,
) (
	*unstructured.Unstructured,
	error,
) {
	if ref == nil {
		return nil, errors.New("reference is not set")
	}

	obj, err := external.Get(ctx, cli, ref)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get %s %s/%s", ref.Kind, ref.Name, ref.Namespace)
	}
	return obj, nil
}

func walkReferences(
	ctx context.Context,
	cc *clusterv1.ClusterClass,
	fn func(ctx context.Context,
		ref *corev1.ObjectReference,
	) error,
) error {
	for _, ref := range []*corev1.ObjectReference{
		cc.Spec.Infrastructure.Ref,
		cc.Spec.ControlPlane.Ref,
		cc.Spec.ControlPlane.MachineInfrastructure.Ref,
	} {
		if err := fn(ctx, ref); err != nil {
			return err
		}
	}

	for mdIdx := range cc.Spec.Workers.MachineDeployments {
		md := &cc.Spec.Workers.MachineDeployments[mdIdx]

		for _, ref := range []*corev1.ObjectReference{
			md.Template.Infrastructure.Ref,
			md.Template.Bootstrap.Ref,
		} {
			if err := fn(ctx, ref); err != nil {
				return err
			}
		}
	}

	return nil
}
