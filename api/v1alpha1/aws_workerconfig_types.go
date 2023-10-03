// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

type AWSWorkerSpec struct{}

var AWSWorkerSpecProperties = map[string]clusterv1.JSONSchemaProps{}

func (AWSWorkerSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS worker configuration",
			Type:        "object",
			Properties:  AWSWorkerSpecProperties,
		},
	}
}
