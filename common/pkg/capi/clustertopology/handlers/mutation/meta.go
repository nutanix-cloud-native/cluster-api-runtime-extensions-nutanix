// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sync"

	"dario.cat/mergo"
	"github.com/samber/lo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
)

type MutateFunc func(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey client.ObjectKey,
) error

type ClusterGetter func(context.Context) (*clusterv1.Cluster, error)

type MetaMutator interface {
	Mutate(
		ctx context.Context,
		obj *unstructured.Unstructured,
		vars map[string]apiextensionsv1.JSON,
		holderRef runtimehooksv1.HolderReference,
		clusterKey client.ObjectKey,
		getCluster ClusterGetter,
	) error
}

type metaGeneratePatches struct {
	name     string
	mutators []MetaMutator
	cl       client.Client
}

func NewMetaGeneratePatchesHandler(
	name string,
	cl client.Client,
	mutators ...MetaMutator,
) handlers.Named {
	return metaGeneratePatches{
		name:     name,
		cl:       cl,
		mutators: mutators,
	}
}

func (mgp metaGeneratePatches) Name() string {
	return mgp.name
}

func (mgp metaGeneratePatches) CreateClusterGetter(
	clusterKey client.ObjectKey,
) func(context.Context) (*clusterv1.Cluster, error) {
	return func(ctx context.Context) (*clusterv1.Cluster, error) {
		var (
			cluster clusterv1.Cluster
			err     error
			once    sync.Once
		)
		once.Do(func() {
			err = mgp.cl.Get(ctx, clusterKey, &cluster)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch cluster: %w", err)
		}
		return &cluster, nil
	}
}

func (mgp metaGeneratePatches) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	clusterKey := handlers.ClusterKeyFromReq(req)
	clusterGetter := mgp.CreateClusterGetter(clusterKey)

	// Create a map of variables from the request that can be overridden by machine deployment or control plane
	// configuration.
	// Filter out the "builtin" variable, which is already present in the vars map and should not be overridden by
	// the global variables.
	globalVars := lo.FilterSliceToMap(
		req.Variables,
		func(v runtimehooksv1.Variable) (string, apiextensionsv1.JSON, bool) {
			if v.Name == "builtin" {
				return "", apiextensionsv1.JSON{}, false
			}
			return v.Name, v.Value, true
		},
	)

	topologymutation.WalkTemplates(
		ctx,
		unstructured.UnstructuredJSONScheme,
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			log := ctrl.LoggerFrom(ctx).WithValues(
				"holderRef", holderRef,
				"objectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"handlerName", mgp.name,
			)

			log.V(3).Info("Starting mutation pipeline", "handlerCount", len(mgp.mutators))

			// Merge the global variables to the current resource vars. This allows the handlers to access
			// the variables defined in the cluster class or cluster configuration and use these correctly when
			// overrides are specified on machine deployment or control plane configuration.
			mergedVars, err := mergeVariableDefinitions(vars, globalVars)
			if err != nil {
				log.Error(err, "Failed to merge global variables")
				return err
			}

			for i, h := range mgp.mutators {
				mutatorType := fmt.Sprintf("%T", h)
				log.V(5).
					Info("Running mutator", "index", i, "handler", mutatorType, "vars", vars)

				if err := h.Mutate(
					ctx,
					obj.(*unstructured.Unstructured),
					mergedVars,
					holderRef,
					clusterKey,
					clusterGetter,
				); err != nil {
					log.Error(err, "Mutator failed", "index", i, "handler", mutatorType)
					return err
				}

				log.V(5).Info("Mutator completed successfully", "index", i, "handler", mutatorType)
			}

			log.V(3).Info("Mutation pipeline completed successfully", "handlerCount", len(mgp.mutators))
			return nil
		},
	)
}

func mergeVariableDefinitions(
	vars, globalVars map[string]apiextensionsv1.JSON,
) (map[string]apiextensionsv1.JSON, error) {
	mergedVars := maps.Clone(vars)

	for k, v := range globalVars {
		// If the value of v is nil, skip it.
		if v.Raw == nil {
			continue
		}

		existingValue, exists := mergedVars[k]

		// If the variable does not exist in the mergedVars or the value is nil, add it and continue.
		if !exists || existingValue.Raw == nil {
			mergedVars[k] = v
			continue
		}

		// Wrap the value in a temporary key to ensure we can unmarshal to a map.
		// This is necessary because the values could be scalars.
		tempValJSON := fmt.Sprintf(`{"value": %s}`, string(existingValue.Raw))
		tempGlobalValJSON := fmt.Sprintf(`{"value": %s}`, string(v.Raw))

		// Unmarshal the existing value and the global value into maps.
		var val, globalVal map[string]interface{}
		if err := json.Unmarshal([]byte(tempValJSON), &val); err != nil {
			return nil, fmt.Errorf("failed to unmarshal existing value for key %q: %w", k, err)
		}

		if err := json.Unmarshal([]byte(tempGlobalValJSON), &globalVal); err != nil {
			return nil, fmt.Errorf("failed to unmarshal global value for key %q: %w", k, err)
		}

		// Now use mergo to perform a deep merge of the values, retaining the values in `val` if present.
		if err := mergo.Merge(&val, globalVal); err != nil {
			return nil, fmt.Errorf("failed to merge values for key %q: %w", k, err)
		}

		// Marshal the merged value back to JSON.
		mergedVal, err := json.Marshal(val["value"])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal merged value for key %q: %w", k, err)
		}

		mergedVars[k] = apiextensionsv1.JSON{Raw: mergedVal}
	}

	return mergedVars, nil
}
