// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package capitest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/openapi"
)

type VariableTestDef struct {
	Name        string
	Vals        any
	ExpectError bool
}

func ValidateDiscoverVariables[T mutation.DiscoverVariables](
	t *testing.T,
	variableName string,
	variableSchema *clusterv1.VariableSchema,
	variableRequired bool,
	handlerCreator func() T,
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

	variable := resp.Variables[0]
	g.Expect(variable).To(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Name":     gomega.Equal(variableName),
		"Required": gomega.Equal(variableRequired),
		"Schema":   gomega.Equal(*variableSchema),
	}))

	for _, tt := range variableTestDefs {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			encodedVals, err := json.Marshal(tt.Vals)
			g.Expect(err).NotTo(gomega.HaveOccurred())

			validateErr := openapi.ValidateClusterVariable(
				&clusterv1.ClusterVariable{
					Name:  variableName,
					Value: apiextensionsv1.JSON{Raw: encodedVals},
				},
				&variable,
				field.NewPath(variableName),
			).ToAggregate()

			if tt.ExpectError {
				g.Expect(validateErr).To(gomega.HaveOccurred())
			} else {
				g.Expect(validateErr).NotTo(gomega.HaveOccurred())
			}
		})
	}
}
