// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package auditpolicy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/auditpolicy"
)

func TestGeneratePatches(t *testing.T) {
	g := NewWithT(t)
	h := auditpolicy.NewPatch()
	req := &runtimehooksv1.GeneratePatchesRequest{}
	resp := &runtimehooksv1.GeneratePatchesResponse{}
	h.GeneratePatches(context.Background(), req, resp)
	g.Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))
}

func TestGeneratePatches_KubeadmControlPlaneTemplate(t *testing.T) {
	g := NewWithT(t)
	h := auditpolicy.NewPatch()
	req := &runtimehooksv1.GeneratePatchesRequest{
		Items: []runtimehooksv1.GeneratePatchesRequestItem{
			requestItem(
				"1",
				&controlplanev1.KubeadmControlPlaneTemplate{
					TypeMeta: v1.TypeMeta{
						Kind:       "KubeadmControlPlaneTemplate",
						APIVersion: controlplanev1.GroupVersion.String(),
					},
				},
				&runtimehooksv1.HolderReference{
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
			ConsistOf(
				MatchAllFields(Fields{
					"Operation": Equal("add"),
					"Path":      Equal("/spec/template/spec/kubeadmConfigSpec/files"),
					"Value":     HaveLen(1),
				}),
				jsonpatch.NewOperation(
					"add",
					"/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					map[string]interface{}{
						"scheduler": map[string]interface{}{},
						"apiServer": map[string]interface{}{
							"extraArgs": map[string]interface{}{
								"audit-log-maxbackup": "10",
								"audit-log-maxsize":   "100",
								"audit-log-path":      "/var/log/audit/kube-apiserver-audit.log",
								"audit-policy-file":   "/etc/kubernetes/audit-policy/apiserver-audit-policy.yaml",
								"audit-log-maxage":    "30",
							},
							"extraVolumes": []interface{}{
								map[string]interface{}{
									"hostPath":  "/etc/kubernetes/audit-policy/",
									"mountPath": "/etc/kubernetes/audit-policy/",
									"name":      "audit-policy",
									"readOnly":  true,
								},
								map[string]interface{}{
									"name":      "audit-logs",
									"hostPath":  "/var/log/kubernetes/audit",
									"mountPath": "/var/log/audit/",
								},
							},
						},
						"controllerManager": map[string]interface{}{},
						"dns":               map[string]interface{}{},
						"etcd":              map[string]interface{}{},
						"networking":        map[string]interface{}{},
					},
				),
			),
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
	holderRef *runtimehooksv1.HolderReference,
) runtimehooksv1.GeneratePatchesRequestItem {
	return runtimehooksv1.GeneratePatchesRequestItem{
		UID: types.UID(uid),
		Object: runtime.RawExtension{
			Raw: toJSON(object),
		},
		HolderReference: *holderRef,
	}
}
