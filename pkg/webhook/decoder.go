// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// DecodeCluster decodes the admission request into a v1beta1.Cluster.
// It handles both v1beta1 and v1beta2 API versions by decoding from raw JSON
// which works for both versions since the core Cluster fields are compatible.
func DecodeCluster(decoder admission.Decoder, req admission.Request) (*clusterv1.Cluster, error) {
	cluster := &clusterv1.Cluster{}

	// Decode from raw JSON - this works for both v1beta1 and v1beta2 requests
	// since the JSON field names are compatible.
	if err := json.Unmarshal(req.Object.Raw, cluster); err != nil {
		return nil, fmt.Errorf("failed to decode cluster from raw JSON: %w", err)
	}

	return cluster, nil
}

// DecodeClusterRaw decodes raw extension bytes into a v1beta1.Cluster.
// It handles both v1beta1 and v1beta2 API versions.
func DecodeClusterRaw(decoder admission.Decoder, raw runtime.RawExtension) (*clusterv1.Cluster, error) {
	cluster := &clusterv1.Cluster{}

	// Decode from raw JSON - this works for both v1beta1 and v1beta2
	if err := json.Unmarshal(raw.Raw, cluster); err != nil {
		return nil, fmt.Errorf("failed to decode cluster from raw JSON: %w", err)
	}

	return cluster, nil
}

// PatchRawAnnotation modifies the raw JSON to add/update an annotation and returns a patch response.
// This preserves all fields in the original object (including v1beta2-specific fields)
// by working directly with the raw JSON instead of marshaling a Go struct.
func PatchRawAnnotation(rawObject []byte, key, value string) admission.Response {
	var obj map[string]interface{}
	if err := json.Unmarshal(rawObject, &obj); err != nil {
		return admission.Errored(500, fmt.Errorf("failed to unmarshal raw object: %w", err))
	}

	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		metadata = make(map[string]interface{})
		obj["metadata"] = metadata
	}

	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		annotations = make(map[string]interface{})
		metadata["annotations"] = annotations
	}

	annotations[key] = value

	modifiedRaw, err := json.Marshal(obj)
	if err != nil {
		return admission.Errored(500, fmt.Errorf("failed to marshal modified object: %w", err))
	}

	return admission.PatchResponseFromRaw(rawObject, modifiedRaw)
}

// PatchRawTopologyVariables modifies the raw JSON to update spec.topology.variables
// and returns a patch response. This preserves all fields in the original object
// (including v1beta2-specific fields) by working directly with the raw JSON.
func PatchRawTopologyVariables(rawObject []byte, variables interface{}) admission.Response {
	var obj map[string]interface{}
	if err := json.Unmarshal(rawObject, &obj); err != nil {
		return admission.Errored(500, fmt.Errorf("failed to unmarshal raw object: %w", err))
	}

	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return admission.Errored(500, fmt.Errorf("spec not found in object"))
	}

	topology, ok := spec["topology"].(map[string]interface{})
	if !ok {
		return admission.Errored(500, fmt.Errorf("spec.topology not found in object"))
	}

	variablesJSON, err := json.Marshal(variables)
	if err != nil {
		return admission.Errored(500, fmt.Errorf("failed to marshal variables: %w", err))
	}

	var variablesInterface interface{}
	if err := json.Unmarshal(variablesJSON, &variablesInterface); err != nil {
		return admission.Errored(500, fmt.Errorf("failed to unmarshal variables: %w", err))
	}

	topology["variables"] = variablesInterface

	modifiedRaw, err := json.Marshal(obj)
	if err != nil {
		return admission.Errored(500, fmt.Errorf("failed to marshal modified object: %w", err))
	}

	return admission.PatchResponseFromRaw(rawObject, modifiedRaw)
}
