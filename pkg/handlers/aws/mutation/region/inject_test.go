// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package region

import (
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
)

func newAWSClusterTemplateRequestItem(uid types.UID) runtimehooksv1.GeneratePatchesRequestItem {
	return request.NewRequestItem(
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1beta2",
				"kind":       "AWSClusterTemplate",
				"metadata": map[string]interface{}{
					"name":      "aws-cluster-template",
					"namespace": "aws-cluster",
				},
			},
		},
		&runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			FieldPath:  "spec.infrastructureRef",
		},
		uid,
	)
}

func TestGeneratePatches(t *testing.T) {
	capitest.ValidateGeneratePatches(
		t,
		func() mutation.GeneratePatches { return NewPatch() },
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
			Name: "region set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.RegionUSWest1,
				),
			},
			RequestItem: newAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec",
				ValueMatcher: gomega.Equal(map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"region": string(v1alpha1.RegionUSWest1),
						},
					},
				}),
			}},
		},
	)
}
