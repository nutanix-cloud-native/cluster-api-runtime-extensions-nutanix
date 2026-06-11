//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigsyaml "sigs.k8s.io/yaml"
)

func TestEnableAutomaticReservationsInClusterTemplate(t *testing.T) {
	const template = `apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: ${CLUSTER_NAME}
spec:
  topology:
    classRef:
      name: nutanix-quick-start
    variables:
    - name: clusterConfig
      value:
        controlPlane: {}
    - name: workerConfig
      value:
        nutanix:
          machineDetails:
            memorySize: 4Gi
            vcpuSockets: 2
    version: ${KUBERNETES_VERSION}
    workers:
      machineDeployments:
      - class: default-worker
        metadata:
          annotations:
            cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: "${WORKER_MACHINE_COUNT}"
        name: md-0
        variables:
          overrides:
          - name: workerConfig
            value:
              nutanix:
                machineDetails:
                  memorySize: 8Gi
`

	patched, err := enableAutomaticReservationsInClusterTemplate([]byte(template))
	require.NoError(t, err)

	// Regression guard: quoted envsubst placeholders (e.g. annotation values that must remain
	// strings) must keep their quotes, otherwise after substitution they parse as integers and
	// break conversion of string-typed fields such as metadata.annotations.
	assert.Contains(t, string(patched), `"${WORKER_MACHINE_COUNT}"`,
		"quoted placeholders must remain quoted after patching")

	obj := map[string]interface{}{}
	require.NoError(t, sigsyaml.Unmarshal(patched, &obj))
	u := unstructured.Unstructured{Object: obj}

	clusterVars, found, err := unstructured.NestedSlice(u.Object, "spec", "topology", "variables")
	require.NoError(t, err)
	require.True(t, found)

	workerConfig := findVariable(t, clusterVars, "workerConfig")
	assert.Equal(t,
		"CapacityTiered",
		nestedString(t, workerConfig, "value", "kubeletConfiguration", "automaticReservations", "profile"),
		"cluster-level workerConfig should enable automatic reservations",
	)
	assert.Equal(t,
		"4Gi",
		nestedString(t, workerConfig, "value", "nutanix", "machineDetails", "memorySize"),
		"existing provider machine details must be preserved",
	)

	clusterConfig := findVariable(t, clusterVars, "clusterConfig")
	_, hasKubelet, err := unstructured.NestedMap(clusterConfig, "value", "kubeletConfiguration")
	require.NoError(t, err)
	assert.False(t, hasKubelet, "clusterConfig must not be modified")

	version := nestedString(t, u.Object, "spec", "topology", "version")
	assert.Equal(t, "${KUBERNETES_VERSION}", version, "envsubst placeholders must survive the round trip")

	mds, found, err := unstructured.NestedSlice(
		u.Object, "spec", "topology", "workers", "machineDeployments",
	)
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, mds, 1)
	md, ok := mds[0].(map[string]interface{})
	require.True(t, ok)
	overrides, found, err := unstructured.NestedSlice(md, "variables", "overrides")
	require.NoError(t, err)
	require.True(t, found)
	overrideWorkerConfig := findVariable(t, overrides, "workerConfig")
	assert.Equal(t,
		"CapacityTiered",
		nestedString(t, overrideWorkerConfig, "value", "kubeletConfiguration", "automaticReservations", "profile"),
		"per-MachineDeployment override should also enable automatic reservations",
	)
	assert.Equal(t,
		"8Gi",
		nestedString(t, overrideWorkerConfig, "value", "nutanix", "machineDetails", "memorySize"),
		"override provider machine details must be preserved",
	)
}

func TestEnableAutomaticReservationsRejectsNonCluster(t *testing.T) {
	_, err := enableAutomaticReservationsInClusterTemplate([]byte("kind: ConfigMap\napiVersion: v1\n"))
	require.Error(t, err)
}

func TestEnableKubeletInUserNamespaceInClusterClass(t *testing.T) {
	const clusterClass = `apiVersion: controlplane.cluster.x-k8s.io/v1beta2
kind: KubeadmControlPlaneTemplate
metadata:
  name: docker-quick-start-control-plane
spec:
  template:
    spec:
      kubeadmConfigSpec:
        initConfiguration:
          nodeRegistration:
            kubeletExtraArgs:
            - name: eviction-hard
              value: nodefs.available<0%
        joinConfiguration:
          nodeRegistration:
            kubeletExtraArgs:
            - name: eviction-hard
              value: nodefs.available<0%
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta2
kind: KubeadmConfigTemplate
metadata:
  name: docker-quick-start-default-worker-bootstraptemplate
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          kubeletExtraArgs:
          - name: eviction-hard
            value: nodefs.available<0%
`

	patched, err := enableKubeletInUserNamespaceInClusterClass([]byte(clusterClass))
	require.NoError(t, err)

	// The gate must be applied to all three kubeletExtraArgs lists (CP init, CP join, worker join).
	assert.Equal(t, 3, strings.Count(string(patched), kubeletInUserNamespaceFeatureGate),
		"feature gate should be present in every kubeletExtraArgs list")
	// Existing args must be preserved alongside the new gate.
	assert.Equal(t, 3, strings.Count(string(patched), "eviction-hard"),
		"existing kubeletExtraArgs must be preserved")
	// The multi-document structure must be preserved (separators emitted, both docs decodable).
	assert.Contains(t, string(patched), "kind: KubeadmControlPlaneTemplate")
	assert.Contains(t, string(patched), "kind: KubeadmConfigTemplate")
	dec := yaml.NewDecoder(bytes.NewReader(patched))
	docCount := 0
	for {
		var n yaml.Node
		err := dec.Decode(&n)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		docCount++
	}
	assert.Equal(t, 2, docCount, "patched output must remain two YAML documents")
}

func TestEnableKubeletInUserNamespaceExtendsExistingFeatureGates(t *testing.T) {
	const clusterClass = `apiVersion: bootstrap.cluster.x-k8s.io/v1beta2
kind: KubeadmConfigTemplate
metadata:
  name: worker
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          kubeletExtraArgs:
          - name: feature-gates
            value: SomeOtherGate=true
`

	patched, err := enableKubeletInUserNamespaceInClusterClass([]byte(clusterClass))
	require.NoError(t, err)

	out := string(patched)
	assert.Contains(t, out, "SomeOtherGate=true", "existing feature gates must be preserved")
	assert.Contains(t, out, "KubeletInUserNamespace=true", "new gate must be appended")
	assert.Equal(t, 1, strings.Count(out, "name: feature-gates"),
		"feature gates must be merged into a single entry, not duplicated")
}

func findVariable(t *testing.T, vars []interface{}, name string) map[string]interface{} {
	t.Helper()
	for _, v := range vars {
		vm, ok := v.(map[string]interface{})
		if ok && vm["name"] == name {
			return vm
		}
	}
	t.Fatalf("variable %q not found", name)
	return nil
}

func nestedString(t *testing.T, obj map[string]interface{}, fields ...string) string {
	t.Helper()
	s, found, err := unstructured.NestedString(obj, fields...)
	require.NoError(t, err)
	require.True(t, found, "expected string at %v", fields)
	return s
}
