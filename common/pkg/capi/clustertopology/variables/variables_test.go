// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables_test

import (
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
)

func TestGet(t *testing.T) {
	g := NewWithT(t)

	type sampleStruct struct {
		Foo string `json:"foo"`
	}
	sampleValue := []byte(`{"foo": "bar"}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	parsed, found, err := variables.Get[sampleStruct](vars, "sampleVar")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(parsed).To(Equal(sampleStruct{
		Foo: "bar",
	}))
}

func TestGetVariable_NotFound(t *testing.T) {
	g := NewWithT(t)

	vars := map[string]apiextensionsv1.JSON{}
	parsed, found, err := variables.Get[string](vars, "not_found")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeFalse())
	g.Expect(parsed).To(BeEmpty())
}

func TestGetVariable_ParseError(t *testing.T) {
	g := NewWithT(t)

	vars := map[string]apiextensionsv1.JSON{
		"intvar": {Raw: []byte("10")},
	}
	parsed, found, err := variables.Get[string](vars, "intvar")
	g.Expect(err).To(HaveOccurred())
	g.Expect(found).To(BeFalse())
	g.Expect(parsed).To(BeEmpty())
}

func TestGet_ValidNestedFieldAsStruct(t *testing.T) {
	g := NewWithT(t)

	type nestedStruct struct {
		Bar string `json:"bar"`
	}
	sampleValue := []byte(`{"foo": {"bar": "baz"}}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	parsed, found, err := variables.Get[nestedStruct](vars, "sampleVar", "foo")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(parsed).To(Equal(nestedStruct{
		Bar: "baz",
	}))
}

func TestGet_ValidNestedFieldAsPrimitive(t *testing.T) {
	g := NewWithT(t)

	sampleValue := []byte(`{"foo": {"bar": "baz"}}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	parsed, found, err := variables.Get[string](vars, "sampleVar", "foo", "bar")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(parsed).To(Equal("baz"))
}

func TestGet_InvalidNestedFieldType(t *testing.T) {
	g := NewWithT(t)

	sampleValue := []byte(`{"foo": {"bar": "baz"}}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	_, _, err := variables.Get[int](vars, "sampleVar", "foo", "bar")
	g.Expect(err).To(HaveOccurred())
}

func TestGet_MissingNestedField(t *testing.T) {
	g := NewWithT(t)

	sampleValue := []byte(`{"foo": {"bar": "baz"}}`)
	vars := map[string]apiextensionsv1.JSON{
		"sampleVar": {Raw: sampleValue},
	}
	_, found, err := variables.Get[string](vars, "sampleVar", "foo", "nonexistent")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeFalse())
}
