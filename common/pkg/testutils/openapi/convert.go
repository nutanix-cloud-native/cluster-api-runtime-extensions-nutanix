// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"encoding/json"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ConvertJSONSchemaPropsToAPIExtensions converts a clusterv1.JSONSchemaProps to apiextensions.JSONSchemaProp.
// NOTE: This is used whenever we want to use one of the upstream libraries, as they use apiextensions.JSONSchemaProp.
// NOTE: If new fields are added to clusterv1.JSONSchemaProps (e.g. to support complex types), the corresponding
// schema validation must be added to validateRootSchema too.
// See: https://github.com/kubernetes-sigs/cluster-api/blob/v1.5.1/internal/topology/variables/schema.go#L35
func ConvertJSONSchemaPropsToAPIExtensions(
	schema *clusterv1.JSONSchemaProps, fldPath *field.Path,
) (*apiextensions.JSONSchemaProps, field.ErrorList) {
	var allErrs field.ErrorList

	props := &apiextensions.JSONSchemaProps{
		Type:             schema.Type,
		Required:         schema.Required,
		MaxItems:         schema.MaxItems,
		MinItems:         schema.MinItems,
		UniqueItems:      schema.UniqueItems,
		Format:           schema.Format,
		MaxLength:        schema.MaxLength,
		MinLength:        schema.MinLength,
		Pattern:          schema.Pattern,
		ExclusiveMaximum: schema.ExclusiveMaximum,
		ExclusiveMinimum: schema.ExclusiveMinimum,
	}

	// Only set XPreserveUnknownFields to true if it's true.
	// apiextensions.JSONSchemaProps only allows setting XPreserveUnknownFields
	// to true or undefined, false is forbidden.
	if schema.XPreserveUnknownFields {
		props.XPreserveUnknownFields = ptr.To(true)
	}

	if schema.Default != nil && schema.Default.Raw != nil {
		var v interface{}
		if err := json.Unmarshal(schema.Default.Raw, &v); err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(
					fldPath.Child("default"),
					string(schema.Default.Raw),
					fmt.Sprintf("default is not valid JSON: %v", err),
				),
			)
		} else {
			var v apiextensions.JSON
			err := apiextensionsv1.Convert_v1_JSON_To_apiextensions_JSON(schema.Default, &v, nil)
			if err != nil {
				allErrs = append(
					allErrs,
					field.Invalid(
						fldPath.Child("default"),
						string(schema.Default.Raw),
						fmt.Sprintf("failed to convert default: %v", err),
					),
				)
			} else {
				props.Default = &v
			}
		}
	}

	if len(schema.Enum) > 0 {
		for i, enum := range schema.Enum {
			if enum.Raw == nil {
				continue
			}

			var v interface{}
			if err := json.Unmarshal(enum.Raw, &v); err != nil {
				allErrs = append(
					allErrs,
					field.Invalid(fldPath.Child("enum").Index(i), string(enum.Raw),
						fmt.Sprintf("enum value is not valid JSON: %v", err)),
				)
			} else {
				var v apiextensions.JSON
				err := apiextensionsv1.Convert_v1_JSON_To_apiextensions_JSON(&schema.Enum[i], &v, nil)
				if err != nil {
					allErrs = append(
						allErrs,
						field.Invalid(
							fldPath.Child("enum").Index(i),
							string(enum.Raw),
							fmt.Sprintf("failed to convert enum value: %v", err),
						),
					)
				} else {
					props.Enum = append(props.Enum, v)
				}
			}
		}
	}

	if schema.Example != nil && schema.Example.Raw != nil {
		var v interface{}
		if err := json.Unmarshal(schema.Example.Raw, &v); err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(fldPath.Child("example"), string(schema.Example.Raw),
					fmt.Sprintf("example is not valid JSON: %v", err)),
			)
		} else {
			var value apiextensions.JSON
			err := apiextensionsv1.Convert_v1_JSON_To_apiextensions_JSON(schema.Example, &value, nil)
			if err != nil {
				allErrs = append(
					allErrs,
					field.Invalid(
						fldPath.Child("example"),
						string(schema.Example.Raw),
						fmt.Sprintf("failed to convert example value: %v", err),
					),
				)
			} else {
				props.Example = &value
			}
		}
	}
	if schema.Maximum != nil {
		f := float64(*schema.Maximum)
		props.Maximum = &f
	}

	if schema.Minimum != nil {
		f := float64(*schema.Minimum)
		props.Minimum = &f
	}

	if schema.AdditionalProperties != nil {
		apiExtensionsSchema, err := ConvertJSONSchemaPropsToAPIExtensions(
			schema.AdditionalProperties, fldPath.Child("additionalProperties"),
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
			props.AdditionalProperties = &apiextensions.JSONSchemaPropsOrBool{
				// Allows must be true to allow "additional properties".
				// Otherwise only the ones from .Properties are allowed.
				Allows: true,
				Schema: apiExtensionsSchema,
			}
		}
	}

	if len(schema.Properties) > 0 {
		props.Properties = map[string]apiextensions.JSONSchemaProps{}
		for propertyName := range schema.Properties {
			p := schema.Properties[propertyName]
			apiExtensionsSchema, err := ConvertJSONSchemaPropsToAPIExtensions(
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

	if schema.Items != nil {
		apiExtensionsSchema, err := ConvertJSONSchemaPropsToAPIExtensions(
			schema.Items,
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
			props.Items = &apiextensions.JSONSchemaPropsOrArray{
				Schema: apiExtensionsSchema,
			}
		}
	}

	if schema.XValidations != nil {
		props.XValidations = make([]apiextensions.ValidationRule, 0, len(schema.XValidations))
		for _, v := range schema.XValidations {
			var reason *apiextensions.FieldValueErrorReason
			if v.Reason != "" {
				reason = ptr.To(apiextensions.FieldValueErrorReason(v.Reason))
			}
			props.XValidations = append(props.XValidations, apiextensions.ValidationRule{
				Rule:              v.Rule,
				Message:           v.Message,
				MessageExpression: v.MessageExpression,
				Reason:            reason,
				FieldPath:         v.FieldPath,
			})
		}
	}

	return props, allErrs
}
