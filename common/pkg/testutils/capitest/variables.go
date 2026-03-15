// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package capitest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/openapi"
)

type VariableTestDef struct {
	Name        string
	Vals        any
	OldVals     any
	ExpectError bool
}

func ValidateDiscoverVariables[V mutation.DiscoverVariables](
	t *testing.T,
	variableName string,
	variableSchema *clusterv1.VariableSchema,
	variableRequired bool,
	handlerCreator func() V,
	variableTestDefs ...VariableTestDef,
) {
	t.Helper()

	ValidateDiscoverVariablesAs[V, any](
		t,
		variableName,
		variableSchema,
		variableRequired,
		handlerCreator,
		variableTestDefs...,
	)
}

func ValidateDiscoverVariablesAs[V mutation.DiscoverVariables, T any](
	t *testing.T,
	variableName string,
	variableSchema *clusterv1.VariableSchema,
	variableRequired bool,
	handlerCreator func() V,
	variableTestDefs ...VariableTestDef,
) {
	t.Helper()

	t.Parallel()

	g := gomega.NewWithT(t)
	h := handlerCreator()
	resp := &runtimehooksv1.DiscoverVariablesResponse{}
	h.DiscoverVariables(
		context.Background(),
		&runtimehooksv1.DiscoverVariablesRequest{},
		resp,
	)

	g.Expect(resp.Status).To(gomega.Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(resp.Variables).To(gomega.HaveLen(1))

	variableV1Beta1 := resp.Variables[0]
	var variableV1Beta2 clusterv1.ClusterClassVariable
	_ = clusterv1beta1.Convert_v1beta1_ClusterClassVariable_To_v1beta2_ClusterClassVariable(
		&variableV1Beta1,
		&variableV1Beta2,
		nil,
	)
	g.Expect(variableV1Beta1.Name).To(gomega.Equal(variableName))
	g.Expect(variableV1Beta1.Required).To(gomega.Equal(variableRequired))
	// Sanity check schema type; skip full equality: v1beta2→v1beta1→v1beta2 round-trip
	// loses semantic equivalence (e.g. nil vs ptr.To(false), XMetadata). VariableTestDef
	// validation below exercises schema behavior.
	g.Expect(variableV1Beta2.Schema.OpenAPIV3Schema.Type).To(gomega.Equal(variableSchema.OpenAPIV3Schema.Type))

	for _, tt := range variableTestDefs {
		t.Run(tt.Name, func(t *testing.T) {
			t.Helper()

			t.Parallel()

			g := gomega.NewWithT(t)

			encodedVals, err := json.Marshal(tt.Vals)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			var validateErr error

			switch {
			case tt.OldVals != nil:
				encodedOldVals, err := json.Marshal(tt.OldVals)
				g.Expect(err).NotTo(gomega.HaveOccurred())
				validateErr = openapi.ValidateClusterVariableUpdate[T](
					&clusterv1.ClusterVariable{
						Name:  variableName,
						Value: apiextensionsv1.JSON{Raw: encodedVals},
					},
					&clusterv1.ClusterVariable{
						Name:  variableName,
						Value: apiextensionsv1.JSON{Raw: encodedOldVals},
					},
					&variableV1Beta2,
					field.NewPath(variableName),
				).ToAggregate()
			default:
				validateErr = openapi.ValidateClusterVariable[T](
					&clusterv1.ClusterVariable{
						Name:  variableName,
						Value: apiextensionsv1.JSON{Raw: encodedVals},
					},
					&variableV1Beta2,
					field.NewPath(variableName),
				).ToAggregate()
			}

			if tt.ExpectError {
				g.Expect(validateErr).To(gomega.HaveOccurred())
			} else {
				g.Expect(validateErr).NotTo(gomega.HaveOccurred())
			}
		})
	}
}
