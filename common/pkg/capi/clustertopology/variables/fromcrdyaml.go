// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
		ExclusiveMaximum:       schema.ExclusiveMaximum,
		ExclusiveMinimum:       schema.ExclusiveMinimum,
		XPreserveUnknownFields: ptr.Deref(schema.XPreserveUnknownFields, false),
		Default:                schema.Default,
		Enum:                   schema.Enum,
		Example:                schema.Example,
	}

	if schema.Maximum != nil {
		props.Maximum = ptr.To(int64(*schema.Maximum))
	}

	if schema.Minimum != nil {
		props.Minimum = ptr.To(int64(*schema.Minimum))
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

	return props, allErrs
}
