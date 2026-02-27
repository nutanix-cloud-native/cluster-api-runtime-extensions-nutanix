// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"fmt"
	"math"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

func MustSchemaFromCRDYAML(yaml []byte) clusterv1.VariableSchema {
	schema, err := SchemaFromCRDYAML(yaml)
	if err != nil {
		panic(err)
	}
	return schema
}

func SchemaFromCRDYAML(yaml []byte) (clusterv1.VariableSchema, error) {
	sch := runtime.NewScheme()
	utilruntime.Must(apiextensionsv1.AddToScheme(sch))
	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode
	obj, gKV, _ := decode(yaml, nil, nil)
	if gKV.Kind != "CustomResourceDefinition" {
		return clusterv1.VariableSchema{}, fmt.Errorf(
			"expected CustomResourceDefinition, got %s",
			gKV.Kind,
		)
	}
	crd := obj.(*apiextensionsv1.CustomResourceDefinition)
	if len(crd.Spec.Versions) != 1 {
		return clusterv1.VariableSchema{}, fmt.Errorf(
			"expected exactly one version, got %d",
			len(crd.Spec.Versions),
		)
	}
	if crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
		return clusterv1.VariableSchema{}, fmt.Errorf("expected OpenAPIV3Schema, got nil")
	}

	spec, ok := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
	if !ok {
		return clusterv1.VariableSchema{}, fmt.Errorf("missing spec")
	}

	jsonSchemaProps, err := ConvertAPIExtensionsToJSONSchemaProps(
		&spec, field.NewPath(""),
	)
	if err != nil {
		return clusterv1.VariableSchema{}, fmt.Errorf(
			"failed to parse CRD into variables schema: %w",
			err.ToAggregate(),
		)
	}

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: *jsonSchemaProps,
	}, nil
}

// ConvertAPIExtensionsToJSONSchemaProps converts a apiextensions.JSONSchemaProp to clusterv1.JSONSchemaProps.
func ConvertAPIExtensionsToJSONSchemaProps(
	schema *apiextensionsv1.JSONSchemaProps, fldPath *field.Path,
) (*clusterv1.JSONSchemaProps, field.ErrorList) {
	var allErrs field.ErrorList

	// Check if minimum/maximum is a whole number that we can convert to int64 without loss of precision.
	var maximumAsInt64Ptr, minimumAsInt64Ptr *int64
	if schema.Maximum != nil {
		maximumAsFloat64 := ptr.Deref(schema.Maximum, 0.0)
		if math.Ceil(maximumAsFloat64) == maximumAsFloat64 {
			maximumAsInt64Ptr = ptr.To(int64(maximumAsFloat64))
		} else {
			allErrs = append(
				allErrs,
				field.Invalid(
					fldPath.Child("maximum"),
					maximumAsFloat64,
					"ClusterClass variables only support a whole number for maximum",
				),
			)
		}
	}
	if schema.Minimum != nil {
		minimumAsFloat64 := ptr.Deref(schema.Minimum, 0.0)
		if math.Ceil(minimumAsFloat64) == minimumAsFloat64 {
			minimumAsInt64Ptr = ptr.To(int64(minimumAsFloat64))
		} else {
			allErrs = append(
				allErrs,
				field.Invalid(
					fldPath.Child("minimum"),
					minimumAsFloat64,
					"ClusterClass variables only support a whole number for minimum",
				),
			)
		}
	}

	props := &clusterv1.JSONSchemaProps{
		Type:                   schema.Type,
		Required:               schema.Required,
		MaxItems:               schema.MaxItems,
		MinItems:               schema.MinItems,
		UniqueItems:            schema.UniqueItems,
		Format:                 schema.Format,
		MaxLength:              schema.MaxLength,
		MinLength:              schema.MinLength,
		Pattern:                schema.Pattern,
		Maximum:                maximumAsInt64Ptr,
		Minimum:                minimumAsInt64Ptr,
		ExclusiveMaximum:       schema.ExclusiveMaximum,
		ExclusiveMinimum:       schema.ExclusiveMinimum,
		XPreserveUnknownFields: ptr.Deref(schema.XPreserveUnknownFields, false),
		Default:                schema.Default,
		Enum:                   schema.Enum,
		Example:                schema.Example,
		Description:            schema.Description,
		MaxProperties:          schema.MaxProperties,
		MinProperties:          schema.MinProperties,
	}

	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		jsonSchemaProps, err := ConvertAPIExtensionsToJSONSchemaProps(
			schema.AdditionalProperties.Schema, fldPath.Child("additionalProperties"),
		)
		if err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					fldPath.Child("additionalProperties"),
					"",
					fmt.Sprintf("failed to convert schema: %v", err),
				),
			)
		} else {
			props.AdditionalProperties = jsonSchemaProps
		}
	}

	if len(schema.Properties) > 0 {
		props.Properties = make(map[string]clusterv1.JSONSchemaProps, len(schema.Properties))
		for propertyName := range schema.Properties {
			p := schema.Properties[propertyName]
			apiExtensionsSchema, err := ConvertAPIExtensionsToJSONSchemaProps(
				&p,
				fldPath.Child("properties").Key(propertyName),
			)
			if err != nil {
				allErrs = append(
					allErrs,
					field.Invalid(
						fldPath.Child("properties").Key(propertyName),
						"",
						fmt.Sprintf("failed to convert schema: %v", err),
					),
				)
			} else {
				props.Properties[propertyName] = *apiExtensionsSchema
			}
		}
	}

	if schema.Items != nil && schema.Items.Schema != nil {
		jsonPropsSchema, err := ConvertAPIExtensionsToJSONSchemaProps(
			schema.Items.Schema,
			fldPath.Child("items"),
		)
		if err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					fldPath.Child("items"),
					"",
					fmt.Sprintf("failed to convert schema: %v", err),
				),
			)
		} else {
			props.Items = jsonPropsSchema
		}
	}

	if schema.XValidations != nil {
		props.XValidations = make([]clusterv1.ValidationRule, 0, len(schema.XValidations))
		for _, v := range schema.XValidations {
			reason := ""
			if v.Reason != nil {
				reason = string(*v.Reason)
			}
			props.XValidations = append(props.XValidations, clusterv1.ValidationRule{
				Rule:              v.Rule,
				Message:           v.Message,
				MessageExpression: v.MessageExpression,
				Reason:            clusterv1.FieldValueErrorReason(reason),
				FieldPath:         v.FieldPath,
			})
		}
	}

	return props, allErrs
}
