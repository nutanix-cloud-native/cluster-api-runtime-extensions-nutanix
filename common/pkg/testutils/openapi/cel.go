// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"context"
	"reflect"
	"strings"

	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// validateCELRecursively recursively validates CEL rules across all schema.Properties.
// validator.Validate() does not traverse nested structs, so we need to do it manually.
// It walks each field of the struct, checking if it has a CEL validation rule,
// and if so, it validates the field using the CEL validator.
func validateCELRecursively(
	ctx context.Context,
	validator *cel.Validator,
	path *field.Path,
	schema *structuralschema.Structural,
	newVal reflect.Value,
	oldVal reflect.Value,
	budget int64,
) field.ErrorList {
	var errs field.ErrorList

	// Prepare values for top-level Validate call
	newIface := reflectValueToInterface(newVal)
	oldIface := reflectValueToInterface(oldVal)

	selfErrs, _ := validator.Validate(ctx, path, schema, newIface, oldIface, budget)
	errs = append(errs, selfErrs...)

	// Dereference pointers
	if newVal.Kind() == reflect.Pointer && !newVal.IsNil() {
		newVal = newVal.Elem()
	}
	if oldVal.Kind() == reflect.Pointer && !oldVal.IsNil() {
		oldVal = oldVal.Elem()
	}

	// Recurse only if newVal is struct
	if newVal.Kind() != reflect.Struct {
		return errs
	}

	typ := newVal.Type()
	for i := 0; i < newVal.NumField(); i++ {
		fieldType := typ.Field(i)
		newFieldVal := newVal.Field(i)

		if fieldType.PkgPath != "" {
			continue // unexported
		}

		// Get old value safely
		var oldFieldVal reflect.Value
		if oldVal.IsValid() && oldVal.Kind() == reflect.Struct {
			oldFieldVal = oldVal.FieldByName(fieldType.Name)
		}

		// Handle embedded fields
		if fieldType.Anonymous && newFieldVal.Kind() == reflect.Struct {
			errs = append(errs,
				validateCELRecursively(ctx, validator, path, schema, newFieldVal, oldFieldVal, budget)...)
			continue
		}

		jsonName := jsonFieldName(fieldType)
		if jsonName == "" {
			continue
		}

		subSchema, okSchema := schema.Properties[jsonName]
		subValidator, okValidator := validator.Properties[jsonName]
		if !okSchema || !okValidator {
			continue
		}

		subPath := path.Child(jsonName)
		errs = append(errs,
			validateCELRecursively(ctx, &subValidator, subPath, &subSchema, newFieldVal, oldFieldVal, budget)...)
	}

	return errs
}

func reflectValueToInterface(v reflect.Value) interface{} {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	return v.Interface()
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return ""
	}
	if tag == "" {
		return field.Name
	}
	return strings.Split(tag, ",")[0]
}
