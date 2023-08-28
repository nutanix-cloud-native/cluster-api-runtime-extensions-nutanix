// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"gomodules.xyz/jsonpatch/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/httpproxy"
)

func TestGeneratePatches(t *testing.T) {
	g := NewWithT(t)
	h := httpproxy.NewPatch()
	req := &runtimehooksv1.GeneratePatchesRequest{}
	resp := &runtimehooksv1.GeneratePatchesResponse{}
	h.GeneratePatches(context.Background(), req, resp)
	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
}

func TestGeneratePatches_KubeadmControlPlaneTemplate(t *testing.T) {
	g := NewWithT(t)
	h := httpproxy.NewPatch()
	req := &runtimehooksv1.GeneratePatchesRequest{
		Variables: []runtimehooksv1.Variable{
			newVariable(httpproxy.VariableName, httpproxy.HTTPProxyVariables{
				HTTP:  "http://example.com",
				HTTPS: "https://example.com",
				NO:    []string{"https://no-proxy.example.com"},
			}),
		},
		Items: []runtimehooksv1.GeneratePatchesRequestItem{
			requestItem(
				"1",
				&controlplanev1.KubeadmControlPlaneTemplate{
					TypeMeta: v1.TypeMeta{
						Kind:       "KubeadmControlPlaneTemplate",
						APIVersion: controlplanev1.GroupVersion.String(),
					},
				},
				runtimehooksv1.HolderReference{
					Kind:      "Cluster",
					FieldPath: "spec.controlPlaneRef",
				},
			),
		},
	}
	resp := &runtimehooksv1.GeneratePatchesResponse{}
	h.GeneratePatches(context.Background(), req, resp)
	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(resp.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{
		"UID":       Equal(types.UID("1")),
		"PatchType": Equal(runtimehooksv1.JSONPatchType),
		"Patch": WithTransform(
			func(data []byte) ([]jsonpatch.Operation, error) {
				operations := []jsonpatch.Operation{}
				if err := json.Unmarshal(data, &operations); err != nil {
					return nil, err
				}
				return operations, nil
			},
			ContainElement(MatchFields(IgnoreExtras, Fields{
				"Operation": Equal("add"),
				"Path":      Equal("/spec/template/spec/kubeadmConfigSpec/files"),
				"Value":     HaveLen(2),
			})),
		),
	})))
}

func TestGeneratePatches_KubeadmConfigTemplate(t *testing.T) {
	g := NewWithT(t)
	h := httpproxy.NewPatch()
	req := &runtimehooksv1.GeneratePatchesRequest{
		Variables: []runtimehooksv1.Variable{
			newVariable(httpproxy.VariableName, httpproxy.HTTPProxyVariables{
				HTTP:  "http://example.com",
				HTTPS: "https://example.com",
				NO:    []string{"https://no-proxy.example.com"},
			}),
			newVariable("builtin", map[string]any{
				"machineDeployment": map[string]any{
					"class": "default-worker",
				},
			}),
		},
		Items: []runtimehooksv1.GeneratePatchesRequestItem{
			requestItem(
				"1",
				&bootstrapv1.KubeadmConfigTemplate{
					TypeMeta: v1.TypeMeta{
						Kind:       "KubeadmConfigTemplate",
						APIVersion: bootstrapv1.GroupVersion.String(),
					},
				},
				runtimehooksv1.HolderReference{
					Kind:      "MachineDeployment",
					FieldPath: "spec.template.spec.infrastructureRef",
				},
			),
		},
	}
	resp := &runtimehooksv1.GeneratePatchesResponse{}
	h.GeneratePatches(context.Background(), req, resp)
	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
	g.Expect(resp.Items).To(ContainElement(MatchFields(IgnoreExtras, Fields{
		"UID":       Equal(types.UID("1")),
		"PatchType": Equal(runtimehooksv1.JSONPatchType),
		"Patch": WithTransform(
			func(data []byte) ([]jsonpatch.Operation, error) {
				operations := []jsonpatch.Operation{}
				if err := json.Unmarshal(data, &operations); err != nil {
					return nil, err
				}
				return operations, nil
			},
			ContainElement(MatchFields(IgnoreExtras, Fields{
				"Operation": Equal("add"),
				"Path":      Equal("/spec/template/spec/files"),
				"Value":     HaveLen(2),
			})),
		),
	})))
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

// requestItem returns a GeneratePatchesRequestItem with the given uid, variables and object.
func requestItem(
	uid string,
	object any,
	holderRef runtimehooksv1.HolderReference,
) runtimehooksv1.GeneratePatchesRequestItem {
	return runtimehooksv1.GeneratePatchesRequestItem{
		UID: types.UID(uid),
		Object: runtime.RawExtension{
			Raw: toJSON(object),
		},
		HolderReference: holderRef,
	}
}

// newVariable returns a runtimehooksv1.Variable with the passed name and value.
func newVariable(name string, value any) runtimehooksv1.Variable {
	return runtimehooksv1.Variable{
		Name:  name,
		Value: apiextensionsv1.JSON{Raw: toJSON(value)},
	}
}
