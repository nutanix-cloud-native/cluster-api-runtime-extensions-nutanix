// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	structuralpruning "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// ValidateClusterVariable validates a clusterVariable.
// See: https://github.com/kubernetes-sigs/cluster-api/blob/v1.5.1/internal/topology/variables/cluster_variable_validation.go#L118
//
//nolint:lll // Adding for URL above, does not work when adding to end of line in a comment block.
func ValidateClusterVariable[T any](
	value *clusterv1.ClusterVariable,
	definition *clusterv1.ClusterClassVariable,
	fldPath *field.Path,
) field.ErrorList {
	validator, apiExtensionsSchema, structuralSchema, err := validatorAndSchemas(fldPath, definition)
	if err != nil {
		return field.ErrorList{err}
	}

	variableValue, err := unmarshalAndDefaultVariableValue[T](fldPath, value, structuralSchema)
	if err != nil {
		return field.ErrorList{err}
	}

	// Validate variable against the schema.
	// NOTE: We're reusing a library func used in CRD validation.
	if err := validation.ValidateCustomResource(fldPath, variableValue, validator); err != nil {
		return err
	}

	var oldVariableValue T
	// Validate variable against the schema using CEL.
	if err := validateCEL[T](fldPath, variableValue, oldVariableValue, structuralSchema); err != nil {
		return err
	}

	return validateUnknownFields(fldPath, value, variableValue, apiExtensionsSchema)
}

func unmarshalAndDefaultVariableValue[T any](
	fldPath *field.Path,
	value *clusterv1.ClusterVariable,
	s *structuralschema.Structural,
) (T, *field.Error) {
	// Parse JSON value.
	var variableValue T
	// Only try to unmarshal the clusterVariable if it is not nil, otherwise the variableValue is nil.
	// Note: A clusterVariable with a nil value is the result of setting the variable value to "null" via YAML.
	if value.Value.Raw != nil {
		dec := json.NewDecoder(bytes.NewReader(value.Value.Raw))
		dec.UseNumber()
		if err := dec.Decode(&variableValue); err != nil {
			return variableValue, field.Invalid(
				fldPath.Child("value"), string(value.Value.Raw),
				fmt.Sprintf("variable %q could not be parsed: %v", value.Name, err),
			)
		}
	}

	// When decoding into interface{} / any, encoding/json produces json.Number (due to UseNumber()).
	// Convert json.Number values into int64/float64 so that CEL validation (which is schema-aware)
	// sees numeric values with compatible Go types.
	//
	// Note: We only need this for untyped values. For typed structs, the decoder already coerces
	// numbers into the correct field types (int32, etc.).
	switch reflect.TypeFor[T]().Kind() {
	case reflect.Interface, reflect.Map, reflect.Slice:
		if converted, didConvert := convertJSONNumbers(any(variableValue)); didConvert {
			variableValue = converted.(T)
		}
	}

	defaulting.Default(variableValue, s)

	return variableValue, nil
}

func convertJSONNumbers(v any) (any, bool) {
	switch x := v.(type) {
	case map[string]any:
		converted := false
		for k, vv := range x {
			if cv, ok := convertJSONNumbers(vv); ok {
				x[k] = cv
				converted = true
			}
		}
		return x, converted
	case []any:
		converted := false
		for i, vv := range x {
			if cv, ok := convertJSONNumbers(vv); ok {
				x[i] = cv
				converted = true
			}
		}
		return x, converted
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i, true
		}
		if f, err := x.Float64(); err == nil {
			return f, true
		}
		return v, false
	default:
		return v, false
	}
}

func validatorAndSchemas(
	fldPath *field.Path, definition *clusterv1.ClusterClassVariable,
) (validation.SchemaValidator, *apiextensions.JSONSchemaProps, *structuralschema.Structural, *field.Error) {
	// Convert schema to Kubernetes APIExtensions Schema.
	apiExtensionsSchema, allErrs := ConvertJSONSchemaPropsToAPIExtensions(
		&definition.Schema.OpenAPIV3Schema, field.NewPath("schema"),
	)
	if len(allErrs) > 0 {
		return nil, nil, nil, field.InternalError(
			fldPath,
			fmt.Errorf(
				"failed to convert schema definition for variable %q; ClusterClass should be checked: %v",
				definition.Name,
				allErrs,
			),
		)
	}

	// Create validator for schema.
	validator, _, err := validation.NewSchemaValidator(apiExtensionsSchema)
	if err != nil {
		return nil, nil, nil, field.InternalError(
			fldPath,
			fmt.Errorf(
				"failed to create schema validator for variable %q; ClusterClass should be checked: %v",
				definition.Name,
				err,
			),
		)
	}

	s, err := structuralschema.NewStructural(apiExtensionsSchema)
	if err != nil {
		return nil, nil, nil, field.InternalError(
			fldPath,
			fmt.Errorf(
				"failed to create structural schema for variable %q; ClusterClass should be checked: %v",
				definition.Name,
				err,
			),
		)
	}

	return validator, apiExtensionsSchema, s, nil
}

func validateCEL[T any](
	fldPath *field.Path,
	variableValue, oldVariableValue T,
	structuralSchema *structuralschema.Structural,
) field.ErrorList {
	// Note: k/k CR validation also uses celconfig.PerCallLimit when creating the validator for a custom resource.
	// The current PerCallLimit gives roughly 0.1 second for each expression validation call.
	celValidator := cel.NewValidator(structuralSchema, false, celconfig.PerCallLimit)
	// celValidation will be nil if there are no CEL validations specified in the schema
	// under `x-kubernetes-validations`.
	if celValidator == nil {
		return nil
	}

	// Note: k/k CRD validation also uses celconfig.RuntimeCELCostBudget for the Validate call.
	// The current RuntimeCELCostBudget gives roughly 1 second for the validation of a variable value.
	if validationErrors := validateCELRecursively(
		context.Background(),
		celValidator,
		fldPath.Child("value"),
		structuralSchema,
		reflect.ValueOf(variableValue),
		reflect.ValueOf(oldVariableValue),
		celconfig.RuntimeCELCostBudget,
	); len(validationErrors) > 0 {
		var allErrs field.ErrorList
		for _, validationError := range validationErrors {
			// Set correct value in the field error. ValidateCustomResource sets the type instead of the value.
			validationError.BadValue = variableValue
			allErrs = append(allErrs, validationError)
		}
		return allErrs
	}

	return nil
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
	variableValue any,
	variableSchema *apiextensions.JSONSchemaProps,
) field.ErrorList {
	// Structural schema pruning does not work with scalar values,
	// so we wrap the schema and the variable in objects.
	// <variable-name>: <variable-value>
	wrappedVariable := map[string]any{
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

// ValidateClusterVariableUpdate validates an update to a clusterVariable.
func ValidateClusterVariableUpdate[T any](
	value, oldValue *clusterv1.ClusterVariable,
	definition *clusterv1.ClusterClassVariable,
	fldPath *field.Path,
) field.ErrorList {
	validator, apiExtensionsSchema, structuralSchema, err := validatorAndSchemas(fldPath, definition)
	if err != nil {
		return field.ErrorList{err}
	}

	variableValue, err := unmarshalAndDefaultVariableValue[T](fldPath, value, structuralSchema)
	if err != nil {
		return field.ErrorList{err}
	}

	oldVariableValue, err := unmarshalAndDefaultVariableValue[T](fldPath, oldValue, structuralSchema)
	if err != nil {
		return field.ErrorList{err}
	}

	// Validate variable against the schema.
	// NOTE: We're reusing a library func used in CRD validation.
	if err := validation.ValidateCustomResourceUpdate(fldPath, variableValue, oldVariableValue, validator); err != nil {
		return err
	}

	// Validate variable against the schema using CEL.
	if err := validateCEL[T](fldPath, variableValue, oldVariableValue, structuralSchema); err != nil {
		return err
	}

	return validateUnknownFields(fldPath, value, variableValue, apiExtensionsSchema)
}
