// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ExtensionHandlers provides a common struct shared across the topology mutation hook handlers.
type ExtensionHandlers struct {
	client ctrlclient.Client
}

// NewExtensionHandlers returns a ExtensionHandlers for the topology mutation hooks handlers.
func NewExtensionHandlers(
	client ctrlclient.Client,
) *ExtensionHandlers {
	return &ExtensionHandlers{
		client: client,
	}
}

func (m *ExtensionHandlers) DoDiscoverVariables(
	ctx context.Context,
	request *runtimehooksv1.DiscoverVariablesRequest,
	response *runtimehooksv1.DiscoverVariablesResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("DiscoverVariables is called")
}

var (
	clusterGK = capiv1beta1.GroupVersion.WithKind("Cluster").GroupKind()

	unstructuredDecoder = serializer.NewCodecFactory(nil).UniversalDeserializer()
)

func (m *ExtensionHandlers) DoGeneratePatches(
	ctx context.Context,
	request *runtimehooksv1.GeneratePatchesRequest,
	response *runtimehooksv1.GeneratePatchesResponse,
) {
	log := ctrl.LoggerFrom(ctx)

	for i := range request.Items {
		gvk := request.Items[i].Object.Object.GetObjectKind().GroupVersionKind()
		if gvk.GroupKind() == clusterGK {
			obj := &unstructured.Unstructured{}
			_, _, err := unstructuredDecoder.Decode(request.Items[i].Object.Raw, nil, obj)
			if err != nil {
				response.Status = runtimehooksv1.ResponseStatusFailure
				response.Message = fmt.Sprintf("failed to decode cluster object: %v", err)
				return
			}

			log = log.WithValues(
				"Cluster",
				obj.GetName(),
				"Namespace",
				obj.GetNamespace(),
			)
			break
		}
	}

	log.Info("GeneratePatches is called")
}

func (m *ExtensionHandlers) DoValidateTopology(
	ctx context.Context,
	request *runtimehooksv1.ValidateTopologyRequest,
	response *runtimehooksv1.ValidateTopologyResponse,
) {
	log := ctrl.LoggerFrom(ctx)

	for i := range request.Items {
		gvk := request.Items[i].Object.Object.GetObjectKind().GroupVersionKind()
		if gvk.GroupKind() == clusterGK {
			obj := &unstructured.Unstructured{}
			_, _, err := unstructuredDecoder.Decode(request.Items[i].Object.Raw, nil, obj)
			if err != nil {
				response.Status = runtimehooksv1.ResponseStatusFailure
				response.Message = fmt.Sprintf("failed to decode cluster object: %v", err)
				return
			}

			log = log.WithValues(
				"Cluster",
				obj.GetName(),
				"Namespace",
				obj.GetNamespace(),
			)
			break
		}
	}

	log.Info("ValidateTopology is called")
}
