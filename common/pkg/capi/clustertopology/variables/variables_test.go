// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"testing"

	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestGet(t *testing.T) {
	g := gomega.NewWithT(t)

	type sampleStruct struct {
		Foo string `json:"foo"`
	}
	sampleValue := []byte(`{"foo": "bar"}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	parsed, err := Get[sampleStruct](vars, "sampleVar")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(parsed).To(gomega.Equal(sampleStruct{
		Foo: "bar",
	}))
}

func TestGetVariable_NotFound(t *testing.T) {
	g := gomega.NewWithT(t)

	vars := map[string]apiextensionsv1.JSON{}
	parsed, err := Get[string](vars, "not_found")
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(IsNotFoundError(err)).To(gomega.BeTrue())
	g.Expect(parsed).To(gomega.BeEmpty())
}

func TestGetVariable_ParseError(t *testing.T) {
	g := gomega.NewWithT(t)

	vars := map[string]apiextensionsv1.JSON{
		"intvar": {Raw: []byte("10")},
	}
	parsed, err := Get[string](vars, "intvar")
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(IsNotFoundError(err)).To(gomega.BeFalse())
	g.Expect(parsed).To(gomega.BeEmpty())
}

func TestGet_ValidNestedFieldAsStruct(t *testing.T) {
	g := gomega.NewWithT(t)

	type nestedStruct struct {
		Bar string `json:"bar"`
	}
	sampleValue := []byte(`{"foo": {"bar": "baz"}}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	parsed, err := Get[nestedStruct](vars, "sampleVar", "foo")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(parsed).To(gomega.Equal(nestedStruct{
		Bar: "baz",
	}))
}

func TestGet_ValidNestedFieldAsPrimitive(t *testing.T) {
	g := gomega.NewWithT(t)

	sampleValue := []byte(`{"foo": {"bar": "baz"}}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	parsed, err := Get[string](vars, "sampleVar", "foo", "bar")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(parsed).To(gomega.Equal("baz"))
}

func TestGet_InvalidNestedFieldType(t *testing.T) {
	g := gomega.NewWithT(t)

	sampleValue := []byte(`{"foo": {"bar": "baz"}}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	parsed, err := Get[int](vars, "sampleVar", "foo", "bar")
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(IsNotFoundError(err)).To(gomega.BeFalse())
	g.Expect(parsed).To(gomega.Equal(0))
}

func TestGet_MissingNestedField(t *testing.T) {
	g := gomega.NewWithT(t)

	sampleValue := []byte(`{"foo": {"bar": "baz"}}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	parsed, err := Get[string](vars, "sampleVar", "foo", "nonexistent")
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(IsNotFoundError(err)).To(gomega.BeTrue())
	g.Expect(parsed).To(gomega.BeEmpty())
}

func TestClusterVariablesToVariablesMap(t *testing.T) {
	t.Parallel()

	g := gomega.NewWithT(t)

	testCases := []struct {
		name        string
		variables   []v1beta1.ClusterVariable
		expectedMap map[string]apiextensionsv1.JSON
	}{{
		name:        "Empty variables",
		variables:   nil,
		expectedMap: nil,
	}, {
		name: "Single variable",
		variables: []v1beta1.ClusterVariable{{
			Name:  "variable1",
			Value: apiextensionsv1.JSON{Raw: []byte(`{"key": "value1"}`)},
		}},
		expectedMap: map[string]apiextensionsv1.JSON{
			"variable1": {Raw: []byte(`{"key": "value1"}`)},
		},
	}, {
		name: "Multiple variables",
		variables: []v1beta1.ClusterVariable{{
			Name:  "variable1",
			Value: apiextensionsv1.JSON{Raw: []byte(`{"key1": "value1"}`)},
		}, {
			Name:  "variable2",
			Value: apiextensionsv1.JSON{Raw: []byte(`{"key2": "value2"}`)},
		}},
		expectedMap: map[string]apiextensionsv1.JSON{
			"variable1": {Raw: []byte(`{"key1": "value1"}`)},
			"variable2": {Raw: []byte(`{"key2": "value2"}`)},
		},
	}}

	for i := range testCases {
		tt := testCases[i]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Call the function under test
			result := ClusterVariablesToVariablesMap(tt.variables)

			// Assert the result
			g.Expect(result).To(gomega.Equal(tt.expectedMap))
		})
	}
}
