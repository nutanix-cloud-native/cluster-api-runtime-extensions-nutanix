// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	bootstrapv2 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	nodemanagerv1alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/node-manager/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

var testScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clusterv1.AddToScheme(s))
	utilruntime.Must(clusterv1beta2.AddToScheme(s))
	utilruntime.Must(bootstrapv2.AddToScheme(s))
	utilruntime.Must(nodemanagerv1alpha1.AddToScheme(s))
	return s
}()

// testClusterWithInPlaceSupport returns a Cluster with the in-place update annotation set (for tests that expect in-place handling).
func testClusterWithInPlaceSupport() *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "c1", Namespace: "default",
			Annotations: map[string]string{v1alpha1.InPlaceUpdateSupportAnnotationKey: "true"},
		},
	}
}

func TestCanUpdateMachine_returnsSuccessAndPatch(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(testClusterWithInPlaceSupport()).Build()
	handler := New(client)

	currentRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "old", Permissions: "0600"},
			},
		},
	})
	desiredRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "new", Permissions: "0600"},
			},
		},
	})

	req := &runtimehooksv1.CanUpdateMachineRequest{
		Current: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine:         clusterv1beta2.Machine{ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: "default"}},
			BootstrapConfig: runtime.RawExtension{Raw: currentRaw},
		},
		Desired: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine: clusterv1beta2.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "m1", Namespace: "default",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "c1"},
				},
			},
			BootstrapConfig: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.CanUpdateMachineResponse{}

	handler.CanUpdateMachine(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusSuccess {
		t.Errorf("status = %v, want Success", resp.Status)
	}
	if !resp.BootstrapConfigPatch.IsDefined() {
		t.Error("BootstrapConfigPatch should be set")
	}
	if resp.BootstrapConfigPatch.PatchType != runtimehooksv1.JSONMergePatchType {
		t.Errorf("PatchType = %v, want JSONMergePatch", resp.BootstrapConfigPatch.PatchType)
	}
}

func TestCanUpdateMachineSet_returnsSuccessAndPatch(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(testClusterWithInPlaceSupport()).Build()
	handler := New(client)

	currentRaw := mustMarshalBootstrapConfigTemplate(bootstrapv2.KubeadmConfigTemplate{
		Spec: bootstrapv2.KubeadmConfigTemplateSpec{
			Template: bootstrapv2.KubeadmConfigTemplateResource{
				Spec: bootstrapv2.KubeadmConfigSpec{
					Files: []bootstrapv2.File{
						{Path: pathImageCredentialProviderConfig, Content: "old", Permissions: "0600"},
					},
				},
			},
		},
	})
	desiredRaw := mustMarshalBootstrapConfigTemplate(bootstrapv2.KubeadmConfigTemplate{
		Spec: bootstrapv2.KubeadmConfigTemplateSpec{
			Template: bootstrapv2.KubeadmConfigTemplateResource{
				Spec: bootstrapv2.KubeadmConfigSpec{
					Files: []bootstrapv2.File{
						{Path: pathImageCredentialProviderConfig, Content: "new", Permissions: "0600"},
					},
				},
			},
		},
	})

	req := &runtimehooksv1.CanUpdateMachineSetRequest{
		Current: runtimehooksv1.CanUpdateMachineSetRequestObjects{
			MachineSet: clusterv1beta2.MachineSet{
				ObjectMeta: metav1.ObjectMeta{Name: "ms1", Namespace: "default"},
			},
			BootstrapConfigTemplate: runtime.RawExtension{Raw: currentRaw},
		},
		Desired: runtimehooksv1.CanUpdateMachineSetRequestObjects{
			MachineSet: clusterv1beta2.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ms1", Namespace: "default",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "c1"},
				},
			},
			BootstrapConfigTemplate: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.CanUpdateMachineSetResponse{}

	handler.CanUpdateMachineSet(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusSuccess {
		t.Errorf("status = %v, want Success", resp.Status)
	}
	if !resp.BootstrapConfigTemplatePatch.IsDefined() {
		t.Error("BootstrapConfigTemplatePatch should be set")
	}
}

func TestUpdateMachine_noNodeTask_createsAndReturnsInProgress(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(testClusterWithInPlaceSupport()).Build()
	handler := New(client)

	desiredRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "content", Permissions: "0600"},
			},
		},
	})

	req := &runtimehooksv1.UpdateMachineRequest{
		Desired: runtimehooksv1.UpdateMachineRequestObjects{
			Machine: clusterv1beta2.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "m1", Namespace: "default",
					Labels:      map[string]string{clusterv1.ClusterNameLabel: "c1"},
					Annotations: map[string]string{v1alpha1.InPlaceUpdateSupportAnnotationKey: "true"},
				},
			},
			BootstrapConfig: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.UpdateMachineResponse{}

	handler.UpdateMachine(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusSuccess {
		t.Errorf("status = %v, want Success", resp.Status)
	}
	if resp.GetRetryAfterSeconds() != retryAfterSecondsInProgress {
		t.Errorf("RetryAfterSeconds = %d, want %d", resp.GetRetryAfterSeconds(), retryAfterSecondsInProgress)
	}
	var list nodemanagerv1alpha1.NodeTaskList
	if err := client.List(ctx, &list); err != nil {
		t.Fatalf("list NodeTasks: %v", err)
	}
	if len(list.Items) != 1 {
		t.Errorf("len(NodeTasks) = %d, want 1", len(list.Items))
	}
	nt := &list.Items[0]
	if nt.Name != "inplace-m1" {
		t.Errorf("NodeTask name = %q, want inplace-m1", nt.Name)
	}
	if nt.Spec.NodeSelector == nil || len(nt.Spec.NodeSelector.NodeNames) != 1 ||
		nt.Spec.NodeSelector.NodeNames[0] != "m1" {
		t.Errorf("NodeSelector.NodeNames = %v, want [m1]", nt.Spec.NodeSelector)
	}
}

func TestUpdateMachine_existingNodeTaskSucceeded_returnsComplete(t *testing.T) {
	ctx := context.Background()
	existing := &nodemanagerv1alpha1.NodeTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inplace-m1", Namespace: "default",
			Labels: map[string]string{inPlaceUpdateMachineLabelKey: "m1", clusterv1.ClusterNameLabel: "c1"},
		},
		Status: nodemanagerv1alpha1.NodeTaskStatus{
			Phase: nodemanagerv1alpha1.NodeTaskPhaseSucceeded,
		},
	}
	client := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(testClusterWithInPlaceSupport(), existing).
		Build()
	handler := New(client)

	desiredRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "x", Permissions: "0600"},
			},
		},
	})
	req := &runtimehooksv1.UpdateMachineRequest{
		Desired: runtimehooksv1.UpdateMachineRequestObjects{
			Machine: clusterv1beta2.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "m1", Namespace: "default",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "c1"},
				},
			},
			BootstrapConfig: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.UpdateMachineResponse{}

	handler.UpdateMachine(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusSuccess {
		t.Errorf("status = %v, want Success", resp.Status)
	}
	if resp.GetRetryAfterSeconds() != 0 {
		t.Errorf("RetryAfterSeconds = %d, want 0", resp.GetRetryAfterSeconds())
	}
}

func TestUpdateMachine_existingNodeTaskFailed_returnsFailure(t *testing.T) {
	ctx := context.Background()
	existing := &nodemanagerv1alpha1.NodeTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inplace-m1", Namespace: "default",
			Labels: map[string]string{inPlaceUpdateMachineLabelKey: "m1", clusterv1.ClusterNameLabel: "c1"},
		},
		Status: nodemanagerv1alpha1.NodeTaskStatus{
			Phase:        nodemanagerv1alpha1.NodeTaskPhaseFailed,
			NodeStatuses: []nodemanagerv1alpha1.NodeStatus{{NodeName: "m1", Error: "task failed"}},
		},
	}
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(existing).Build()
	handler := New(client)

	desiredRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "x", Permissions: "0600"},
			},
		},
	})
	req := &runtimehooksv1.UpdateMachineRequest{
		Desired: runtimehooksv1.UpdateMachineRequestObjects{
			Machine: clusterv1beta2.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "m1", Namespace: "default",
					Labels:      map[string]string{clusterv1.ClusterNameLabel: "c1"},
					Annotations: map[string]string{v1alpha1.InPlaceUpdateSupportAnnotationKey: "true"},
				},
			},
			BootstrapConfig: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.UpdateMachineResponse{}

	handler.UpdateMachine(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusFailure {
		t.Errorf("status = %v, want Failure", resp.Status)
	}
	if resp.Message == "" {
		t.Error("Message should be set on failure")
	}
}

func TestUpdateMachine_existingNodeTaskRunning_returnsInProgress(t *testing.T) {
	ctx := context.Background()
	existing := &nodemanagerv1alpha1.NodeTask{
		ObjectMeta: metav1.ObjectMeta{
			Name: "inplace-m1", Namespace: "default",
			Labels: map[string]string{inPlaceUpdateMachineLabelKey: "m1", clusterv1.ClusterNameLabel: "c1"},
		},
		Status: nodemanagerv1alpha1.NodeTaskStatus{Phase: nodemanagerv1alpha1.NodeTaskPhaseRunning},
	}
	client := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(testClusterWithInPlaceSupport(), existing).
		Build()
	handler := New(client)

	desiredRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "x", Permissions: "0600"},
			},
		},
	})
	req := &runtimehooksv1.UpdateMachineRequest{
		Desired: runtimehooksv1.UpdateMachineRequestObjects{
			Machine: clusterv1beta2.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "m1", Namespace: "default",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "c1"},
				},
			},
			BootstrapConfig: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.UpdateMachineResponse{}

	handler.UpdateMachine(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusSuccess {
		t.Errorf("status = %v, want Success", resp.Status)
	}
	if resp.GetRetryAfterSeconds() != retryAfterSecondsInProgress {
		t.Errorf("RetryAfterSeconds = %d, want %d", resp.GetRetryAfterSeconds(), retryAfterSecondsInProgress)
	}
}

func TestUpdateMachine_missingBootstrap_returnsFailure(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().WithScheme(testScheme).Build()
	handler := New(client)

	req := &runtimehooksv1.UpdateMachineRequest{
		Desired: runtimehooksv1.UpdateMachineRequestObjects{
			Machine:         clusterv1beta2.Machine{ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: "default"}},
			BootstrapConfig: runtime.RawExtension{Raw: nil},
		},
	}
	resp := &runtimehooksv1.UpdateMachineResponse{}

	handler.UpdateMachine(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusFailure {
		t.Errorf("status = %v, want Failure", resp.Status)
	}
	if resp.Message == "" {
		t.Error("Message should be set")
	}
}

// TestCanUpdateMachine_withoutAnnotation_skipsPatch: no Cluster with in-place annotation in client → patch not set.
func TestCanUpdateMachine_withoutAnnotation_skipsPatch(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().WithScheme(testScheme).Build() // no Cluster, so annotation on Cluster is not set
	handler := New(client)

	currentRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "old", Permissions: "0600"},
			},
		},
	})
	desiredRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "new", Permissions: "0600"},
			},
		},
	})

	req := &runtimehooksv1.CanUpdateMachineRequest{
		Current: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine:         clusterv1beta2.Machine{ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: "default"}},
			BootstrapConfig: runtime.RawExtension{Raw: currentRaw},
		},
		Desired: runtimehooksv1.CanUpdateMachineRequestObjects{
			Machine: clusterv1beta2.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "m1", Namespace: "default",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
				},
			},
			BootstrapConfig: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.CanUpdateMachineResponse{}

	handler.CanUpdateMachine(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusSuccess {
		t.Errorf("status = %v, want Success", resp.Status)
	}
	if resp.BootstrapConfigPatch.IsDefined() {
		t.Error(
			"BootstrapConfigPatch should not be set when Cluster does not have in-place annotation; CAPI will roll out new machines",
		)
	}
}

// TestUpdateMachine_withoutAnnotation_returnsFailure: Cluster does not have in-place annotation → failure.
func TestUpdateMachine_withoutAnnotation_returnsFailure(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().WithScheme(testScheme).Build() // no Cluster with annotation
	handler := New(client)

	desiredRaw := mustMarshalBootstrapConfig(bootstrapv2.KubeadmConfig{
		Spec: bootstrapv2.KubeadmConfigSpec{
			Files: []bootstrapv2.File{
				{Path: pathImageCredentialProviderConfig, Content: "x", Permissions: "0600"},
			},
		},
	})
	req := &runtimehooksv1.UpdateMachineRequest{
		Desired: runtimehooksv1.UpdateMachineRequestObjects{
			Machine: clusterv1beta2.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "m1", Namespace: "default",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "test-cluster"},
				},
			},
			BootstrapConfig: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.UpdateMachineResponse{}

	handler.UpdateMachine(ctx, req, resp)

	if resp.Status != runtimehooksv1.ResponseStatusFailure {
		t.Errorf("status = %v, want Failure", resp.Status)
	}
	if resp.Message == "" {
		t.Error("Message should be set")
	}
	// Message should direct user to set annotation on the Cluster (not the Machine)
	if resp.Message != "" && resp.Status == runtimehooksv1.ResponseStatusFailure {
		if !strings.Contains(resp.Message, "Cluster") {
			t.Errorf("Message should mention Cluster (annotation is on Cluster, not Machine): %q", resp.Message)
		}
	}
}

func TestBootstrapSpecToNodeTaskFiles_filtersInPlacePaths(t *testing.T) {
	spec := bootstrapv2.KubeadmConfigSpec{
		Files: []bootstrapv2.File{
			{Path: "/etc/other/file", Content: "x", Permissions: "0600"},
			{Path: pathImageCredentialProviderConfig, Content: "a", Permissions: "0600"},
			{Path: pathStaticImageCredentials, ContentFrom: bootstrapv2.FileSource{
				Secret: bootstrapv2.SecretFileSource{Name: "s1", Key: "k1"},
			}, Permissions: "0600"},
		},
	}
	got := bootstrapSpecToNodeTaskFilesV2(spec)
	if len(got) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(got))
	}
	if got[0].Path != pathImageCredentialProviderConfig || got[0].Content != "a" {
		t.Errorf("first file: path=%q content=%q", got[0].Path, got[0].Content)
	}
	if got[1].Path != pathStaticImageCredentials || got[1].ContentFrom == nil ||
		got[1].ContentFrom.SecretRef.Name != "s1" {
		t.Errorf("second file: path=%q contentFrom=%+v", got[1].Path, got[1].ContentFrom)
	}
}

func TestBootstrapSpecToNodeTaskFiles_emptyWhenNoInPlacePaths(t *testing.T) {
	spec := bootstrapv2.KubeadmConfigSpec{
		Files: []bootstrapv2.File{
			{Path: "/etc/other/file", Content: "x", Permissions: "0600"},
		},
	}
	got := bootstrapSpecToNodeTaskFilesV2(spec)
	if len(got) != 0 {
		t.Errorf("len(files) = %d, want 0", len(got))
	}
}

func mustMarshalBootstrapConfig(c bootstrapv2.KubeadmConfig) []byte {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return b
}

func mustMarshalBootstrapConfigTemplate(c bootstrapv2.KubeadmConfigTemplate) []byte {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return b
}

func TestSpecFromBootstrapRaw(t *testing.T) {
	raw := []byte(`{"apiVersion":"v1","kind":"KubeadmConfig","spec":{"files":[{"path":"/a","content":"x"}]}}`)
	spec, err := specFromBootstrapRaw(raw)
	if err != nil {
		t.Fatalf("specFromBootstrapRaw: %v", err)
	}
	files, _ := spec["files"].([]interface{})
	if len(files) != 1 {
		t.Errorf("spec.files len = %d, want 1", len(files))
	}
}

func TestSpecFromTemplateRaw(t *testing.T) {
	raw := []byte(`{"apiVersion":"v1","kind":"KubeadmConfigTemplate","spec":{"template":{"spec":{"files":[]}}}}`)
	spec, err := specFromTemplateRaw(raw)
	if err != nil {
		t.Fatalf("specFromTemplateRaw: %v", err)
	}
	template, _ := spec["template"].(map[string]interface{})
	if template == nil {
		t.Fatal("spec.template missing")
	}
}

func TestSpecsEqual(t *testing.T) {
	a := map[string]interface{}{"x": "1"}
	b := map[string]interface{}{"x": "1"}
	if !specsEqual(a, b) {
		t.Error("specsEqual(a,b) want true")
	}
	c := map[string]interface{}{"x": "2"}
	if specsEqual(a, c) {
		t.Error("specsEqual(a,c) want false")
	}
	if !specsEqual(nil, nil) {
		t.Error("specsEqual(nil,nil) want true")
	}
}

// TestCanUpdateMachineSet_identicalSpec_noPatch verifies we do not set a patch when current and desired spec are identical.
func TestCanUpdateMachineSet_identicalSpec_noPatch(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(testClusterWithInPlaceSupport()).Build()
	handler := New(client)
	sameRaw := mustMarshalBootstrapConfigTemplate(bootstrapv2.KubeadmConfigTemplate{
		Spec: bootstrapv2.KubeadmConfigTemplateSpec{
			Template: bootstrapv2.KubeadmConfigTemplateResource{
				Spec: bootstrapv2.KubeadmConfigSpec{
					Files: []bootstrapv2.File{
						{Path: pathImageCredentialProviderConfig, Content: "same", Permissions: "0600"},
					},
				},
			},
		},
	})
	req := &runtimehooksv1.CanUpdateMachineSetRequest{
		Current: runtimehooksv1.CanUpdateMachineSetRequestObjects{
			MachineSet: clusterv1beta2.MachineSet{
				ObjectMeta: metav1.ObjectMeta{Name: "ms1", Namespace: "default"},
			},
			BootstrapConfigTemplate: runtime.RawExtension{Raw: sameRaw},
		},
		Desired: runtimehooksv1.CanUpdateMachineSetRequestObjects{
			MachineSet: clusterv1beta2.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ms1", Namespace: "default",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "c1"},
				},
			},
			BootstrapConfigTemplate: runtime.RawExtension{Raw: sameRaw},
		},
	}
	resp := &runtimehooksv1.CanUpdateMachineSetResponse{}
	handler.CanUpdateMachineSet(ctx, req, resp)
	if resp.Status != runtimehooksv1.ResponseStatusSuccess {
		t.Errorf("status = %v, want Success", resp.Status)
	}
	if resp.BootstrapConfigTemplatePatch.IsDefined() {
		t.Error("BootstrapConfigTemplatePatch should not be set when current and desired spec are identical")
	}
}

// TestDecodeV1beta2_kubeletExtraArgsArray verifies that v1beta2 bootstrap config with
// kubeletExtraArgs as array (native v1beta2 format) decodes correctly.
func TestDecodeV1beta2_kubeletExtraArgsArray(t *testing.T) {
	raw := []byte(`{
		"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta2",
		"kind": "KubeadmConfigTemplate",
		"spec": {
			"template": {
				"spec": {
					"joinConfiguration": {
						"nodeRegistration": {
							"kubeletExtraArgs": [
								{"name": "cloud-provider", "value": "external"},
								{"name": "some-arg", "value": "some-val"}
							]
						}
					},
					"files": []
				}
			}
		}
	}`)
	var tmpl bootstrapv2.KubeadmConfigTemplate
	if err := json.Unmarshal(raw, &tmpl); err != nil {
		t.Fatalf("decode v1beta2 template: %v", err)
	}
	jc := tmpl.Spec.Template.Spec.JoinConfiguration
	nr := jc.NodeRegistration
	if len(nr.KubeletExtraArgs) != 2 {
		t.Fatalf("kubeletExtraArgs len = %d, want 2", len(nr.KubeletExtraArgs))
	}
	var foundCloud, foundSome bool
	for _, a := range nr.KubeletExtraArgs {
		val := ""
		if a.Value != nil {
			val = *a.Value
		}
		if a.Name == "cloud-provider" && val == "external" {
			foundCloud = true
		}
		if a.Name == "some-arg" && val == "some-val" {
			foundSome = true
		}
	}
	if !foundCloud || !foundSome {
		t.Errorf("kubeletExtraArgs = %v, want cloud-provider=external and some-arg=some-val", nr.KubeletExtraArgs)
	}
}

// TestCanUpdateMachineSet_v1beta1WithArray_returnsPatch verifies that when current/desired are v1beta1
// (or have kubeletExtraArgs as array), we normalize, decode as v1beta1, and return a patch so CAPI does not roll out.
func TestCanUpdateMachineSet_v1beta1WithArray_returnsPatch(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(testClusterWithInPlaceSupport()).Build()
	handler := New(client)
	// Raw v1beta1 template with kubeletExtraArgs as array (would fail v1beta2 decode; we fall back to v1beta1 path).
	currentRaw := []byte(`{
		"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
		"kind": "KubeadmConfigTemplate",
		"spec": {
			"template": {
				"spec": {
					"joinConfiguration": {
						"nodeRegistration": {
							"kubeletExtraArgs": [{"name": "x", "value": "y"}]
						}
					},
					"files": [{"path": "` + pathImageCredentialProviderConfig + `", "content": "old", "permissions": "0600"}]
				}
			}
		}
	}`)
	desiredRaw := []byte(`{
		"apiVersion": "bootstrap.cluster.x-k8s.io/v1beta1",
		"kind": "KubeadmConfigTemplate",
		"spec": {
			"template": {
				"spec": {
					"joinConfiguration": {
						"nodeRegistration": {
							"kubeletExtraArgs": [{"name": "x", "value": "y"}]
						}
					},
					"files": [{"path": "` + pathImageCredentialProviderConfig + `", "content": "new", "permissions": "0600"}]
				}
			}
		}
	}`)
	req := &runtimehooksv1.CanUpdateMachineSetRequest{
		Current: runtimehooksv1.CanUpdateMachineSetRequestObjects{
			MachineSet: clusterv1beta2.MachineSet{
				ObjectMeta: metav1.ObjectMeta{Name: "ms1", Namespace: "default"},
			},
			BootstrapConfigTemplate: runtime.RawExtension{Raw: currentRaw},
		},
		Desired: runtimehooksv1.CanUpdateMachineSetRequestObjects{
			MachineSet: clusterv1beta2.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ms1", Namespace: "default",
					Labels: map[string]string{clusterv1.ClusterNameLabel: "c1"},
				},
			},
			BootstrapConfigTemplate: runtime.RawExtension{Raw: desiredRaw},
		},
	}
	resp := &runtimehooksv1.CanUpdateMachineSetResponse{}
	handler.CanUpdateMachineSet(ctx, req, resp)
	if resp.Status != runtimehooksv1.ResponseStatusSuccess {
		t.Errorf("status = %v, want Success", resp.Status)
	}
	if !resp.BootstrapConfigTemplatePatch.IsDefined() {
		t.Error("BootstrapConfigTemplatePatch should be set for v1beta1-with-array so CAPI does not roll out")
	}
}
