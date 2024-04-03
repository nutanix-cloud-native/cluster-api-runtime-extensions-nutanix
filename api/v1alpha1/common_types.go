// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	APIServerPort = 6443
)

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create. This is a copy of customizable fields from metav1.ObjectMeta.
//
// For more details on why this is included instead of using metav1.ObjectMeta directly, see
// https://github.com/kubernetes-sigs/cluster-api/blob/v1.3.3/api/v1beta1/common_types.go#L175-L195.
type ObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ControlPlaneEndpointSpec clusterv1.APIEndpoint

func (ControlPlaneEndpointSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Kubernetes control-plane endpoint configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"host": {
					Description: "host ip/fqdn for control plane API Server",
					Type:        "string",
					MinLength:   ptr.To[int64](1),
				},
				"port": {
					Description: "port for control plane API Server",
					Type:        "integer",
					Default:     variables.MustMarshal(APIServerPort),
					Minimum:     ptr.To[int64](1),
					Maximum:     ptr.To[int64](65535),
				},
			},
			Required: []string{"host", "port"},
		},
	}
}
