// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package capitest

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"gomodules.xyz/jsonpatch/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/serializer"
)

type PatchTestDef struct {
	Name                  string
	Vars                  []runtimehooksv1.Variable
	RequestItem           runtimehooksv1.GeneratePatchesRequestItem
	ExpectedPatchMatchers []JSONPatchMatcher
	// UnexpectedPatchMatchers used to test patches that should not be present
	UnexpectedPatchMatchers []JSONPatchMatcher
	ExpectedFailure         bool
}

type JSONPatchMatcher struct {
	Operation    string
	Path         string
	ValueMatcher gomegatypes.GomegaMatcher
}

func ValidateGeneratePatches[T mutation.GeneratePatches](
	t GinkgoTInterface,
	handlerCreator func() T,
	testDefs ...PatchTestDef,
) {
	t.Helper()
	testFunc := func(tt *PatchTestDef) {
		g := gomega.NewWithT(t)
		h := handlerCreator()
		req := &runtimehooksv1.GeneratePatchesRequest{
			Variables: tt.Vars,
			Items: []runtimehooksv1.GeneratePatchesRequestItem{
				tt.RequestItem,
				{
					HolderReference: runtimehooksv1.HolderReference{
						APIVersion: capiv1.GroupVersion.String(),
						Kind:       "Cluster",
						Namespace:  request.Namespace,
						Name:       request.ClusterName,
					},
				},
			},
		}
		resp := &runtimehooksv1.GeneratePatchesResponse{}
		h.GeneratePatches(context.Background(), req, resp)
		expectedStatus := runtimehooksv1.ResponseStatusSuccess
		if tt.ExpectedFailure {
			expectedStatus = runtimehooksv1.ResponseStatusFailure
		}
		g.Expect(resp.Status).
			To(gomega.Equal(expectedStatus), fmt.Sprintf("Message: %s", resp.Message))

		if len(tt.ExpectedPatchMatchers) == 0 {
			g.Expect(resp.Items).To(gomega.BeEmpty())
			return
		}
		g.Expect(resp.Items).To(containPatches(&tt.RequestItem, tt.ExpectedPatchMatchers...))

		if len(tt.UnexpectedPatchMatchers) > 0 {
			g.Expect(resp.Items).
				ToNot(containPatches(&tt.RequestItem, tt.UnexpectedPatchMatchers...))
		}
	}

	// compose Ginkgo table arguments
	// https://onsi.github.io/ginkgo/#table-specs for more details
	testArgs := make([]TableEntry, 0, len(testDefs))
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		testArgs = append(testArgs, Entry(tt.Name, &tt))
	}
	DescribeTable("Patches", testFunc, testArgs)
}

// VariableWithValue returns a runtimehooksv1.Variable with the passed name and value.
func VariableWithValue(
	variableName string,
	value any,
	variablePath ...string,
) runtimehooksv1.Variable {
	if len(variablePath) > 0 {
		rootValue := make(map[string]any, 1)
		nestedValue := rootValue

		for _, p := range variablePath[:len(variablePath)-1] {
			nestedValue[p] = make(map[string]any, 1)
			nestedValue = nestedValue[p].(map[string]any)
		}

		nestedValue[variablePath[len(variablePath)-1]] = value
		value = rootValue
	}

	return runtimehooksv1.Variable{
		Name:  variableName,
		Value: apiextensionsv1.JSON{Raw: serializer.ToJSON(value)},
	}
}

func containPatches(
	requestItem *runtimehooksv1.GeneratePatchesRequestItem,
	jsonMatchers ...JSONPatchMatcher,
) gomega.OmegaMatcher {
	patchMatchers := make([]interface{}, 0, len(jsonMatchers))
	for patchIdx := range jsonMatchers {
		unexpectedPatch := jsonMatchers[patchIdx]
		patchMatchers = append(patchMatchers, gstruct.MatchAllFields(gstruct.Fields{
			"Operation": gomega.Equal(unexpectedPatch.Operation),
			"Path":      gomega.Equal(unexpectedPatch.Path),
			"Value":     unexpectedPatch.ValueMatcher,
		}))
	}

	return gomega.ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"UID":       gomega.Equal(requestItem.UID),
		"PatchType": gomega.Equal(runtimehooksv1.JSONPatchType),
		"Patch": gomega.WithTransform(
			func(data []byte) ([]jsonpatch.Operation, error) {
				operations := []jsonpatch.Operation{}
				if err := json.Unmarshal(data, &operations); err != nil {
					return nil, err
				}
				return operations, nil
			},
			gomega.ContainElements(patchMatchers...),
		),
	}))
}
