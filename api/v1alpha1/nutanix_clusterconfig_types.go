// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type NutanixSpec struct{}

func (NutanixSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix cluster configuration",
			Type:        "object",
			Properties:  map[string]clusterv1.JSONSchemaProps{},
		},
	}
}
