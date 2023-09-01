// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/openapi"
)

func TestVariableValidation_dnsImageRepository(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		val         string
		expectError bool
	}{{
		name: "valid image repo",
		val:  "a.b.c.example.com",
	}, {
		name: "valid image repo with port",
		val:  "a.b.c.example.com:1234",
	}, {
		name: "valid image repo with single level path",
		val:  "a.b.c.example.com/abc",
	}, {
		name: "valid image repo with multiple level path",
		val:  "a.b.c.example.com/abc/def/ghi",
	}, {
		name: "valid image repo with multiple level path and port",
		val:  "a.b.c.example.com:1234/abc/def/ghi",
	}}

	for idx := range tests {
		tt := tests[idx]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			h := NewVariable()
			resp := &runtimehooksv1.DiscoverVariablesResponse{}
			h.DiscoverVariables(
				context.Background(),
				&runtimehooksv1.DiscoverVariablesRequest{},
				resp,
			)

			g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
			g.Expect(resp.Variables).To(HaveLen(1))

			variable := resp.Variables[0]
			g.Expect(variable).To(MatchFields(IgnoreExtras, Fields{
				"Name":     Equal(VariableName),
				"Required": BeFalse(),
				"Schema":   Equal(DNSVariable{}.VariableSchema()),
			}))

			encodedVals, err := json.Marshal(DNSVariable{
				ImageRepository: tt.val,
			})
			g.Expect(err).NotTo(HaveOccurred())

			validateErr := openapi.ValidateClusterVariable(
				&clusterv1.ClusterVariable{
					Name:  VariableName,
					Value: apiextensionsv1.JSON{Raw: encodedVals},
				},
				&variable,
				field.NewPath(VariableName),
			).ToAggregate()

			if tt.expectError {
				g.Expect(validateErr).To(HaveOccurred())
			} else {
				g.Expect(validateErr).NotTo(HaveOccurred())
			}
		})
	}
}
