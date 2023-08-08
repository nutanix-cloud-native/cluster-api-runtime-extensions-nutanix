// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/k8s/parser"
)

func dummyUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

var tests = []struct {
	name    string
	inputs  []string
	want    []client.Object
	wantErr string
}{{
	name:   "empty input, nil slice output",
	inputs: nil,
}, {
	name:   "single empty input, nil slice output",
	inputs: []string{``},
}, {
	name: "malformed YAML input",
	inputs: []string{`a: b
b: d
c`},
	wantErr: "could not find expected ':'",
}, {
	name:    "malformed JSON input",
	inputs:  []string{`{"a": "b", "c": "d"`},
	wantErr: "unexpected EOF",
}, {
	name: "valid YAML, but not valid k8s object",
	inputs: []string{`a: b
c: d`},
	wantErr: "Object 'Kind' is missing in",
}, {
	name:    "valid JSON, but not valid k8s object",
	inputs:  []string{`{"a": "b", "c": "d"}`},
	wantErr: "Object 'Kind' is missing in",
}, {
	name: "valid single k8s object YAML",
	inputs: []string{`apiVersion: some.api/v1
kind: Something
metadata:
  namespace: a
  name: b
`},
	want: []client.Object{dummyUnstructured("some.api/v1", "Something", "a", "b")},
}, {
	name: "valid multiple k8s object YAML",
	inputs: []string{`apiVersion: some.api/v1
kind: Something
metadata:
  namespace: a
  name: b
---
apiVersion: another.api/v1
kind: SomethingElse
metadata:
  namespace: c
  name: d
`},
	want: []client.Object{
		dummyUnstructured("some.api/v1", "Something", "a", "b"),
		dummyUnstructured("another.api/v1", "SomethingElse", "c", "d"),
	},
}, {
	name: "valid multiple k8s object YAML including empty docs",
	inputs: []string{`apiVersion: some.api/v1
kind: Something
metadata:
  namespace: a
  name: b
---
---
apiVersion: another.api/v1
kind: SomethingElse
metadata:
  namespace: c
  name: d
---
`},
	want: []client.Object{
		dummyUnstructured("some.api/v1", "Something", "a", "b"),
		dummyUnstructured("another.api/v1", "SomethingElse", "c", "d"),
	},
}, {
	name: "valid multiple k8s object YAML across multiple inputs",
	inputs: []string{
		`apiVersion: some.api/v1
kind: Something
metadata:
  namespace: a
  name: b
---
apiVersion: another.api/v1
kind: SomethingElse
metadata:
  namespace: c
  name: d`,
		`apiVersion: some.api/v2
kind: Something2
metadata:
  namespace: e
  name: f
---
apiVersion: another.api/v2
kind: SomethingElse2
metadata:
  namespace: g
  name: h`,
	},
	want: []client.Object{
		dummyUnstructured("some.api/v1", "Something", "a", "b"),
		dummyUnstructured("another.api/v1", "SomethingElse", "c", "d"),
		dummyUnstructured("some.api/v2", "Something2", "e", "f"),
		dummyUnstructured("another.api/v2", "SomethingElse2", "g", "h"),
	},
}}

func runTests(t *testing.T, fn func([]string) ([]client.Object, error)) {
	t.Helper()
	t.Parallel()
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := fn(tt.inputs)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeReaderToObjects(t *testing.T) {
	runTests(t, func(inputs []string) ([]client.Object, error) {
		readers := make([]io.Reader, 0, len(inputs))
		for _, s := range inputs {
			readers = append(readers, strings.NewReader(s))
		}
		return parser.DecodeReadersToObjects(readers...)
	})
}

func TestBytesToObjects(t *testing.T) {
	runTests(t, func(inputs []string) ([]client.Object, error) {
		bs := make([][]byte, 0, len(inputs))
		for _, s := range inputs {
			bs = append(bs, []byte(s))
		}
		return parser.BytesToObjects(bs...)
	})
}

func TestStringsToObjects(t *testing.T) {
	runTests(t, func(inputs []string) ([]client.Object, error) {
		return parser.StringsToObjects(inputs...)
	})
}
