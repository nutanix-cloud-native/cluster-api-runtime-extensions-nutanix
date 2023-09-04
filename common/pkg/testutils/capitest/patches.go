// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package capitest

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"gomodules.xyz/jsonpatch/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers/mutation"
)

type PatchTestDef struct {
	Name                  string
	Vars                  []runtimehooksv1.Variable
	RequestItem           runtimehooksv1.GeneratePatchesRequestItem
	ExpectedPatchMatchers []JSONPatchMatcher
}

type JSONPatchMatcher struct {
	Operation    string
	Path         string
	ValueMatcher gomegatypes.GomegaMatcher
}

func ValidateGeneratePatches[T mutation.GeneratePatches](
	t *testing.T,
	handlerCreator func() T,
	testDefs ...PatchTestDef,
) {
	t.Helper()

	t.Parallel()

	for testIdx := range testDefs {
		tt := testDefs[testIdx]

		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)
			h := handlerCreator()
			req := &runtimehooksv1.GeneratePatchesRequest{
				Variables: tt.Vars,
				Items:     []runtimehooksv1.GeneratePatchesRequestItem{tt.RequestItem},
			}
			resp := &runtimehooksv1.GeneratePatchesResponse{}
			h.GeneratePatches(context.Background(), req, resp)
			g.Expect(resp.Status).To(gomega.Equal(runtimehooksv1.ResponseStatusSuccess))

			if len(tt.ExpectedPatchMatchers) == 0 {
				g.Expect(resp.Items).To(gomega.BeEmpty())
				return
			}

			patchMatchers := make([]interface{}, 0, len(tt.ExpectedPatchMatchers))
			for patchIdx := range tt.ExpectedPatchMatchers {
				expectedPatch := tt.ExpectedPatchMatchers[patchIdx]
				patchMatchers = append(patchMatchers, gstruct.MatchAllFields(gstruct.Fields{
					"Operation": gomega.Equal(expectedPatch.Operation),
					"Path":      gomega.Equal(expectedPatch.Path),
					"Value":     expectedPatch.ValueMatcher,
				}))
			}

			g.Expect(resp.Items).
				To(gomega.ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"UID":       gomega.Equal(tt.RequestItem.UID),
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
				})))
		})
	}
}

// v returns a runtimehooksv1.Variable with the passed name and value.
func VariableWithValue(name string, value any) runtimehooksv1.Variable {
	return runtimehooksv1.Variable{
		Name:  name,
		Value: apiextensionsv1.JSON{Raw: toJSON(value)},
	}
}

func toJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	compacted := &bytes.Buffer{}
	if err := json.Compact(compacted, data); err != nil {
		panic(err)
	}
	return compacted.Bytes()
}

// requestItem returns a GeneratePatchesRequestItem with the given variables and object.
func requestItem(
	object any,
	holderRef *runtimehooksv1.HolderReference,
) runtimehooksv1.GeneratePatchesRequestItem {
	return runtimehooksv1.GeneratePatchesRequestItem{
		UID: uuid.NewUUID(),
		Object: runtime.RawExtension{
			Raw: toJSON(object),
		},
		HolderReference: *holderRef,
	}
}

func NewKubeadmConfigTemplateRequestItem() runtimehooksv1.GeneratePatchesRequestItem {
	return requestItem(
		&bootstrapv1.KubeadmConfigTemplate{
			TypeMeta: metav1.TypeMeta{
				Kind:       "KubeadmConfigTemplate",
				APIVersion: bootstrapv1.GroupVersion.String(),
			},
		},
		&runtimehooksv1.HolderReference{
			Kind:      "MachineDeployment",
			FieldPath: "spec.template.spec.infrastructureRef",
		},
	)
}

func NewKubeadmControlPlaneTemplateRequestItem() runtimehooksv1.GeneratePatchesRequestItem {
	return requestItem(
		&controlplanev1.KubeadmControlPlaneTemplate{
			TypeMeta: metav1.TypeMeta{
				Kind:       "KubeadmControlPlaneTemplate",
				APIVersion: controlplanev1.GroupVersion.String(),
			},
		},
		&runtimehooksv1.HolderReference{
			Kind:      "Cluster",
			FieldPath: "spec.controlPlaneRef",
		},
	)
}
