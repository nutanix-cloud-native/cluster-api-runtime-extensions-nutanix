// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestValidateClusterVariableWithTypedStructAndObjectXValidation(t *testing.T) {
	t.Parallel()

	type nestedSpec struct {
		DaysBeforeExpiry int32 `json:"daysBeforeExpiry,omitempty"`
	}
	type sampleConfig struct {
		IssuerA string      `json:"issuerA,omitempty"`
		IssuerB string      `json:"issuerB,omitempty"`
		Nested  *nestedSpec `json:"nested,omitempty"`
	}

	def := &clusterv1.ClusterClassVariable{
		Name: "clusterConfig",
		Schema: clusterv1.VariableSchema{
			OpenAPIV3Schema: clusterv1.JSONSchemaProps{
				Type: "object",
				XValidations: []clusterv1.ValidationRule{
					{
						Rule:    "!has(self.issuerA) || !has(self.issuerB) || self.issuerA != self.issuerB",
						Message: "issuerA and issuerB must not overlap",
					},
				},
				Properties: map[string]clusterv1.JSONSchemaProps{
					"issuerA": {Type: "string"},
					"issuerB": {Type: "string"},
					"nested": {
						Type: "object",
						Properties: map[string]clusterv1.JSONSchemaProps{
							"daysBeforeExpiry": {
								Type: "integer",
								XValidations: []clusterv1.ValidationRule{
									{
										Rule:    "self == 0 || self >= 7",
										Message: "must be 0 or >= 7",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("typed struct passes when object and nested CEL rules hold", func(t *testing.T) {
		t.Parallel()
		errs := ValidateClusterVariable[sampleConfig](
			&clusterv1.ClusterVariable{
				Name:  "clusterConfig",
				Value: apiextensionsv1.JSON{Raw: []byte(`{"nested":{"daysBeforeExpiry":7}}`)},
			},
			def,
			field.NewPath("clusterConfig"),
		)
		require.Empty(t, errs)
	})

	t.Run("typed struct fails when object-level CEL rule fails", func(t *testing.T) {
		t.Parallel()
		errs := ValidateClusterVariable[sampleConfig](
			&clusterv1.ClusterVariable{
				Name: "clusterConfig",
				Value: apiextensionsv1.JSON{
					Raw: []byte(`{"issuerA":"https://same.example.com","issuerB":"https://same.example.com"}`),
				},
			},
			def,
			field.NewPath("clusterConfig"),
		)
		require.NotEmpty(t, errs)
		require.ErrorContains(t, errs.ToAggregate(), "issuerA and issuerB must not overlap")
		require.NotContains(t, errs.ToAggregate().Error(), "invalid data")
	})

	t.Run("typed struct fails when nested scalar CEL rule fails", func(t *testing.T) {
		t.Parallel()
		errs := ValidateClusterVariable[sampleConfig](
			&clusterv1.ClusterVariable{
				Name:  "clusterConfig",
				Value: apiextensionsv1.JSON{Raw: []byte(`{"nested":{"daysBeforeExpiry":1}}`)},
			},
			def,
			field.NewPath("clusterConfig"),
		)
		require.NotEmpty(t, errs)
		require.ErrorContains(t, errs.ToAggregate(), "must be 0 or >= 7")
	})
}
