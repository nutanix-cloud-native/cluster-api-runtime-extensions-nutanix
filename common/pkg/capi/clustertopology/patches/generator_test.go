// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patches

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
)

func TestMutateIfApplicable(t *testing.T) {
	t.Parallel()

	type testSpec[T runtime.Object] struct {
		name          string
		input         *unstructured.Unstructured
		holderRef     *runtimehooksv1.HolderReference
		patchSelector clusterv1.PatchSelector
		mutFn         func(T) error
		expected      *unstructured.Unstructured
	}
	tests := []testSpec[*v1.ConfigMap]{{
		name: "empty input matches holder and selector",
		input: &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
			"kind":       "KubeadmControlPlaneTemplate",
		}},
		holderRef: &runtimehooksv1.HolderReference{
			Kind:      "Cluster",
			FieldPath: "spec.controlPlaneRef",
		},
		patchSelector: selectors.ControlPlane(),
		mutFn: func(obj *v1.ConfigMap) error {
			if obj.Data == nil {
				obj.Data = map[string]string{}
			}
			obj.Data["foo"] = "bar" //nolint:goconst // bar doesn't need to be a const.
			return nil
		},
		expected: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
				"kind":       "KubeadmControlPlaneTemplate",
				"data": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}, {
		name:  "empty input not matching holder and selector",
		input: &unstructured.Unstructured{Object: map[string]interface{}{}},
		holderRef: &runtimehooksv1.HolderReference{
			Kind:      "NotMatching",
			FieldPath: "spec.controlPlaneRef",
		},
		patchSelector: selectors.ControlPlane(),
		mutFn: func(obj *v1.ConfigMap) error {
			if obj.Data == nil {
				obj.Data = map[string]string{}
			}
			obj.Data["foo"] = "bar"
			return nil
		},
		expected: &unstructured.Unstructured{
			Object: map[string]interface{}{},
		},
	}, {
		name: "invalid typed object - ignored",
		input: &unstructured.Unstructured{Object: map[string]interface{}{
			"unknownField": "foo",
		}},
		holderRef: &runtimehooksv1.HolderReference{
			Kind:      "Cluster",
			FieldPath: "spec.controlPlaneRef",
		},
		patchSelector: selectors.ControlPlane(),
		mutFn: func(obj *v1.ConfigMap) error {
			return nil
		},
		expected: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"unknownField": "foo",
			},
		},
	}, {
		name: "check deletion of elements in slice",
		input: &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
			"kind":       "KubeadmControlPlaneTemplate",
			"data": map[string]interface{}{
				"existingFoo": "bar",
			},
		}},
		holderRef: &runtimehooksv1.HolderReference{
			Kind:      "Cluster",
			FieldPath: "spec.controlPlaneRef",
		},
		patchSelector: selectors.ControlPlane(),
		mutFn: func(obj *v1.ConfigMap) error {
			if obj.Data == nil {
				obj.Data = map[string]string{}
			}
			obj.Data["foo"] = "bar"
			delete(obj.Data, "existingFoo")
			return nil
		},
		expected: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "controlplane.cluster.x-k8s.io/v1beta1",
				"kind":       "KubeadmControlPlaneTemplate",
				"data": map[string]interface{}{
					"foo": "bar",
				},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewGomegaWithT(t)

			err := MutateIfApplicable(
				tt.input,
				nil,
				tt.holderRef,
				tt.patchSelector,
				logr.Discard(),
				tt.mutFn,
			)
			g.Expect(err).ToNot(gomega.HaveOccurred())
			g.Expect(tt.input).To(gomega.Equal(tt.expected))
		})
	}
}
