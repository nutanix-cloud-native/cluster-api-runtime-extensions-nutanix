// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patches

import (
	"fmt"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/capi/clustertopology/patches/matchers"
)

func Generate[T runtime.Object](
	obj runtime.Object,
	vars map[string]apiextensionsv1.JSON,
	holderRef *runtimehooksv1.HolderReference,
	patchSelector clusterv1.PatchSelector,
	log logr.Logger,
	mutFn func(T) error,
) error {
	typed, ok := obj.(T)
	if !ok {
		log.V(5).WithValues(
			"objType", fmt.Sprintf("%T", obj),
			"expectedType", fmt.Sprintf("%T", *new(T)),
		).Info("not matching type")
		return nil
	}

	if !matchers.MatchesSelector(patchSelector, obj, holderRef, vars) {
		log.V(5).WithValues("selector", patchSelector).Info("not matching selector")
		return nil
	}

	return mutFn(typed)
}
