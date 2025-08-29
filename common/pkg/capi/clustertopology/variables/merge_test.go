// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"
	"testing"

	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestMergeVariableOverridesWithGlobal(t *testing.T) {
	t.Parallel()

	type args struct {
		vars       map[string]apiextensionsv1.JSON
		globalVars map[string]apiextensionsv1.JSON
	}
	tests := []struct {
		name      string
		args      args
		want      map[string]apiextensionsv1.JSON
		wantErr   bool
		errString string
	}{
		{
			name: "no overlap, globalVars added",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`1`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"b": {Raw: []byte(`2`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`1`)},
				"b": {Raw: []byte(`2`)},
			},
		},
		{
			name: "globalVars value is nil, skipped",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`1`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"b": {Raw: nil},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`1`)},
			},
		},
		{
			name: "existing value is nil, globalVars value used",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: nil},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`2`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`2`)},
			},
		},
		{
			name: "both values are scalars, globalVars ignored",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`1`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`2`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`1`)},
			},
		},
		{
			name: "both values are objects, merged",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1,"y":2}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"y":3,"z":4}`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`{"x":1,"y":2,"z":4}`)},
			},
		},
		{
			name: "both values are objects with nested objects, merged",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1,"y":{"a": 2,"b":{"c": 3}}}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"y":{"a": 2,"b":{"c": 5, "d": 6}},"z":4}`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`{"x":1,"y":{"a": 2,"b":{"c": 3, "d": 6}},"z":4}`)},
			},
		},
		{
			name: "both values are objects with nested objects with vars having nil object explicitly, merged",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1,"y":{"a": 2,"b": null}}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"y":{"a": 2,"b":{"c": 5, "d": 6}},"z":4}`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`{"x":1,"y":{"a": 2,"b":{"c": 5, "d": 6}},"z":4}`)},
			},
		},
		{
			name: "globalVars is scalar, vars is object, keep object",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`2`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`{"x":1}`)},
			},
		},
		{
			name: "vars is scalar, globalVars is object, keep scalar",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`2`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1}`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`2`)},
			},
		},
		{
			name: "invalid JSON in vars",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{invalid}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1}`)},
				},
			},
			wantErr:   true,
			errString: "failed to unmarshal existing value for key \"a\"",
		},
		{
			name: "invalid JSON in globalVars",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{invalid}`)},
				},
			},
			wantErr:   true,
			errString: "failed to unmarshal global value for key \"a\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			got, err := MergeVariableOverridesWithGlobal(tt.args.vars, tt.args.globalVars)
			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.errString))
				return
			}
			g.Expect(err).ToNot(gomega.HaveOccurred())
			// Compare JSON values
			for k, wantVal := range tt.want {
				gotVal, ok := got[k]
				g.Expect(ok).To(gomega.BeTrue(), "missing key %q", k)
				var wantObj, gotObj interface{}
				_ = json.Unmarshal(wantVal.Raw, &wantObj)
				_ = json.Unmarshal(gotVal.Raw, &gotObj)
				g.Expect(gotObj).To(gomega.Equal(wantObj), "key %q", k)
			}
			// Check for unexpected keys
			g.Expect(len(got)).To(gomega.Equal(len(tt.want)))
		})
	}
}
