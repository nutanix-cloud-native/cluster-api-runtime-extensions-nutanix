// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package variables

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestValuesToEnumJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    []interface{}
		expected []interface{}
	}{{
		name:     "Empty input",
		input:    []any{},
		expected: []interface{}{},
	}, {
		name:     "Single value",
		input:    []any{"value1"},
		expected: []interface{}{`"value1"`},
	}, {
		name:     "Multiple values",
		input:    []interface{}{"value1", "value2", "value3"},
		expected: []interface{}{`"value1"`, `"value2"`, `"value3"`},
	}, {
		name:     "Multiple integer values",
		input:    []interface{}{1, 2, 3},
		expected: []interface{}{`1`, `2`, `3`},
	}, {
		name:     "Multiple integer array values",
		input:    []interface{}{[]int{1, 2}, []int{2, 3}, []int{49, 64}},
		expected: []interface{}{`[1,2]`, `[2,3]`, `[49,64]`},
	}, {
		name: "Multiple string array values",
		input: []interface{}{
			[]string{"value1", "value2"},
			[]string{"value2", "value3"},
			[]string{"value49", "value64"},
		},
		expected: []interface{}{
			`["value1","value2"]`,
			`["value2","value3"]`,
			`["value49","value64"]`,
		},
	}}

	for i := range testCases {
		tt := testCases[i]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			// Call the function under test
			jsonData := MustMarshalValuesToEnumJSON(tt.input...)

			// Assert the result
			g.Expect(jsonData).To(gomega.HaveLen(len(tt.expected)))
			for i, expected := range tt.expected {
				g.Expect(string(jsonData[i].Raw)).To(gomega.Equal(expected))
			}
		})
	}
}
