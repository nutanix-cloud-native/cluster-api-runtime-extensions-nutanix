// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	structuralpruning "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ValidateClusterVariable validates a clusterVariable.
// See: https://github.com/kubernetes-sigs/cluster-api/blob/v1.5.1/internal/topology/variables/cluster_variable_validation.go#L118
//
//nolint:lll // Adding for URL above, does not work when adding to end of line in a comment block.
func ValidateClusterVariable(
	value *clusterv1.ClusterVariable,
	definition *clusterv1.ClusterClassVariable,
	fldPath *field.Path,
) field.ErrorList {
	// Parse JSON value.
	var variableValue interface{}
	// Only try to unmarshal the clusterVariable if it is not nil, otherwise the variableValue is nil.
	// Note: A clusterVariable with a nil value is the result of setting the variable value to "null" via YAML.
	if value.Value.Raw != nil {
		if err := json.Unmarshal(value.Value.Raw, &variableValue); err != nil {
			return field.ErrorList{field.Invalid(fldPath.Child("value"), string(value.Value.Raw),
				fmt.Sprintf("variable %q could not be parsed: %v", value.Name, err))}
		}
	}

	// Convert schema to Kubernetes APIExtensions Schema.
	apiExtensionsSchema, allErrs := ConvertToAPIExtensionsJSONSchemaProps(
		&definition.Schema.OpenAPIV3Schema, field.NewPath("schema"),
	)
	if len(allErrs) > 0 {
		return field.ErrorList{field.InternalError(fldPath,
			fmt.Errorf(
				"failed to convert schema definition for variable %q; ClusterClass should be checked: %v",
				definition.Name,
				allErrs,
			),
		)}
	}

	// Create validator for schema.
	validator, _, err := validation.NewSchemaValidator(apiExtensionsSchema)
	if err != nil {
		return field.ErrorList{field.InternalError(fldPath,
			fmt.Errorf(
				"failed to create schema validator for variable %q; ClusterClass should be checked: %v",
				value.Name,
				err,
			),
		)}
	}

	s, err := structuralschema.NewStructural(apiExtensionsSchema)
	if err != nil {
		return field.ErrorList{field.InternalError(fldPath,
			fmt.Errorf(
				"failed to create structural schema for variable %q; ClusterClass should be checked: %v",
				value.Name,
				err,
			),
		)}
	}
	defaulting.Default(variableValue, s)

	// Validate variable against the schema.
	// NOTE: We're reusing a library func used in CRD validation.
	if err := validation.ValidateCustomResource(fldPath, variableValue, validator); err != nil {
		return err
	}

	return validateUnknownFields(fldPath, value, variableValue, apiExtensionsSchema)
}

// validateUnknownFields validates the given variableValue for unknown fields.
// This func returns an error if there are variable fields in variableValue that are not defined in
// variableSchema and if x-kubernetes-preserve-unknown-fields is not set.
// See: https://github.com/kubernetes-sigs/cluster-api/blob/v1.5.1/internal/topology/variables/cluster_variable_validation.go#L158
//
//nolint:lll // Adding for URL above, does not work when adding to end of line in a comment block.
func validateUnknownFields(
	fldPath *field.Path,
	clusterVariable *clusterv1.ClusterVariable,
	variableValue interface{},
	variableSchema *apiextensions.JSONSchemaProps,
) field.ErrorList {
	// Structural schema pruning does not work with scalar values,
	// so we wrap the schema and the variable in objects.
	// <variable-name>: <variable-value>
	wrappedVariable := map[string]interface{}{
		clusterVariable.Name: variableValue,
	}
	// type: object
	// properties:
	//   <variable-name>: <variable-schema>
	wrappedSchema := &apiextensions.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensions.JSONSchemaProps{
			clusterVariable.Name: *variableSchema,
		},
	}
	ss, err := structuralschema.NewStructural(wrappedSchema)
	if err != nil {
		return field.ErrorList{field.Invalid(fldPath, "",
			fmt.Sprintf("failed defaulting variable %q: %v", clusterVariable.Name, err))}
	}

	// Run Prune to check if it would drop any unknown fields.
	opts := structuralschema.UnknownFieldPathOptions{
		// TrackUnknownFieldPaths has to be true so PruneWithOptions returns the unknown fields.
		TrackUnknownFieldPaths: true,
	}
	prunedUnknownFields := structuralpruning.PruneWithOptions(wrappedVariable, ss, false, opts)
	if len(prunedUnknownFields) > 0 {
		// If prune dropped any unknown fields, return an error.
		// This means that not all variable fields have been defined in the variable schema and
		// x-kubernetes-preserve-unknown-fields was not set.
		return field.ErrorList{
			field.Invalid(fldPath, "",
				fmt.Sprintf(
					"failed validation: %q fields are not specified in the variable schema of variable %q",
					strings.Join(prunedUnknownFields, ","),
					clusterVariable.Name,
				),
			),
		}
	}

	return nil
}
