// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package taints

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Generate taints patches for Worker", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewWorkerPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "taints for workers set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.WorkerConfigVariableName,
					[]v1alpha1.Taint{{
						Key:    "key",
						Effect: v1alpha1.TaintEffectNoExecute,
						Value:  "value",
					}},
					VariableName,
				),
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/taints",
				ValueMatcher: gomega.ConsistOf(
					map[string]interface{}{"key": "key", "effect": "NoExecute", "value": "value"},
				),
			}},
		},
	}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
		})
	}
})

func Test_toCoreTaints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		existingTaints []v1.Taint
		newTaints      []v1alpha1.Taint
		want           []v1.Taint
	}{{
		name: "nil new and existing taints",
		want: nil,
	}, {
		name:           "nil new taints with existing taints",
		existingTaints: []v1.Taint{{Key: "key", Effect: v1.TaintEffectNoExecute, Value: "value"}},
		want:           []v1.Taint{{Key: "key", Effect: v1.TaintEffectNoExecute, Value: "value"}},
	}, {
		name: "nil existing taints with new taints",
		newTaints: []v1alpha1.Taint{
			{Key: "key", Effect: v1alpha1.TaintEffectNoExecute, Value: "value"},
		},
		want: []v1.Taint{{Key: "key", Effect: v1.TaintEffectNoExecute, Value: "value"}},
	}, {
		name:           "existing and new taints",
		existingTaints: []v1.Taint{{Key: "key", Effect: v1.TaintEffectNoExecute, Value: "value"}},
		newTaints: []v1alpha1.Taint{
			{Key: "key2", Effect: v1alpha1.TaintEffectNoExecute, Value: "value2"},
		},
		want: []v1.Taint{
			{Key: "key", Effect: v1.TaintEffectNoExecute, Value: "value"},
			{Key: "key2", Effect: v1.TaintEffectNoExecute, Value: "value2"},
		},
	}, {
		name:      "nil existing taints and empty but non-nil new taints",
		newTaints: []v1alpha1.Taint{},
		want:      []v1.Taint{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, toCoreTaints(tt.existingTaints, tt.newTaints))
		})
	}
}
