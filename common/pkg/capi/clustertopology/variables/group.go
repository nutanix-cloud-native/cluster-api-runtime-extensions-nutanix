package variables

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// Variable represents a variable definition that can be groupped into a single
// variable handler.
type Variable interface {
	VariableSchema() clusterv1.VariableSchema
}

// Group represent group of named variables.
type Group map[string]Variable

// VariableSchema allows to represent group of variables as a single variable.
func (group Group) VariableSchema() clusterv1.VariableSchema {
	schema := clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]clusterv1.JSONSchemaProps{},
		},
	}

	for name, v := range group {
		schema.OpenAPIV3Schema.Properties[name] = v.VariableSchema().OpenAPIV3Schema
	}

	return schema
}
