//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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

// TestEnableAutomaticReservationsMultiDocument covers provider templates that prepend other
// documents (e.g. the Nutanix example's credential Secrets) before the Cluster document.
func TestEnableAutomaticReservationsMultiDocument(t *testing.T) {
	const template = `apiVersion: v1
kind: Secret
metadata:
  name: ${CLUSTER_NAME}-pc-creds
stringData:
  credentials: "${NUTANIX_USER}:${NUTANIX_PASSWORD}"
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: ${CLUSTER_NAME}
spec:
  topology:
    classRef:
      name: nutanix-quick-start
    variables:
    - name: workerConfig
      value:
        nutanix:
          machineDetails:
            memorySize: 4Gi
    version: ${KUBERNETES_VERSION}
`

	patched, err := enableAutomaticReservationsInClusterTemplate([]byte(template))
	require.NoError(t, err)

	// The Secret must be passed through unchanged, including its quoted envsubst placeholders.
	assert.Contains(t, string(patched), "kind: Secret")
	assert.Contains(t, string(patched), `"${NUTANIX_USER}:${NUTANIX_PASSWORD}"`)

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

	obj := map[string]interface{}{}
	// Decode only the Cluster document (second) for assertions.
	parts := strings.Split(string(patched), "\n---\n")
	require.Len(t, parts, 2)
	require.NoError(t, sigsyaml.Unmarshal([]byte(parts[1]), &obj))
	u := unstructured.Unstructured{Object: obj}
	clusterVars, found, err := unstructured.NestedSlice(u.Object, "spec", "topology", "variables")
	require.NoError(t, err)
	require.True(t, found)
	workerConfig := findVariable(t, clusterVars, "workerConfig")
	assert.Equal(t,
		"CapacityTiered",
		nestedString(t, workerConfig, "value", "kubeletConfiguration", "automaticReservations", "profile"),
	)
	assert.Equal(t,
		"4Gi",
		nestedString(t, workerConfig, "value", "nutanix", "machineDetails", "memorySize"),
	)
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

func TestIsolateProviderVersionsPreventsSharedMutation(t *testing.T) {
	original := &clusterctl.E2EConfig{
		Providers: []clusterctl.ProviderConfig{
			{
				Name: "docker",
				Versions: []clusterctl.ProviderVersionSource{
					{Files: []clusterctl.Files{{SourcePath: "a.yaml", TargetName: "a.yaml"}}},
				},
			},
		},
	}

	copied := original.DeepCopy()
	isolateProviderVersions(copied, "docker")

	// Mutate the copy the way the helpers do.
	copied.Providers[0].Versions[0].Files = append(
		copied.Providers[0].Versions[0].Files,
		clusterctl.Files{SourcePath: "b.yaml", TargetName: "b.yaml"},
	)
	copied.Providers[0].Versions[0].Files[0].SourcePath = "mutated.yaml"

	assert.Len(t, original.Providers[0].Versions[0].Files, 1,
		"appending to the isolated copy must not grow the original config")
	assert.Equal(t, "a.yaml", original.Providers[0].Versions[0].Files[0].SourcePath,
		"repointing SourcePath on the isolated copy must not mutate the original config")
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

func TestGenerateClusterName(t *testing.T) {
	dockerName := generateClusterName("quick-start-", "docker")
	assert.True(t, strings.HasPrefix(dockerName, "quick-start-"))
	assert.Len(t, dockerName, 64-23)

	nutanixName := generateClusterName("quick-start-", "nutanix")
	assert.True(t, strings.HasPrefix(nutanixName, "quick-start-"))
	assert.Len(t, nutanixName, 63)
}

func TestProbeTCPHostInUse(t *testing.T) {
	t.Run("detects open TCP port", func(t *testing.T) {
		var lc net.ListenConfig
		ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })

		_, openPort, err := net.SplitHostPort(ln.Addr().String())
		require.NoError(t, err)

		var dialer net.Dialer
		inUse, reason := probeTCPHostInUse(
			t.Context(),
			dialer.DialContext,
			"127.0.0.1",
			[]string{openPort},
		)
		assert.True(t, inUse)
		assert.Contains(t, reason, "connected")
	})

	t.Run("detects closed TCP port via ECONNREFUSED", func(t *testing.T) {
		var lc net.ListenConfig
		ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		_, closedPort, err := net.SplitHostPort(ln.Addr().String())
		require.NoError(t, err)
		require.NoError(t, ln.Close())

		var dialer net.Dialer
		inUse, reason := probeTCPHostInUse(
			t.Context(),
			dialer.DialContext,
			"127.0.0.1",
			[]string{closedPort},
		)
		assert.True(t, inUse)
		assert.Contains(t, reason, "connection refused")
	})

	t.Run("treats EHOSTUNREACH and timeout as unused", func(t *testing.T) {
		dial := func(_ context.Context, _, address string) (net.Conn, error) {
			if strings.HasSuffix(address, ":6443") {
				return nil, syscall.EHOSTUNREACH
			}
			return nil, context.DeadlineExceeded
		}

		inUse, reason := probeTCPHostInUse(
			t.Context(),
			dial,
			"10.23.133.210",
			[]string{"6443", "80"},
		)
		assert.False(t, inUse)
		assert.Empty(t, reason)
	})
}

func TestReserveUnusedIP(t *testing.T) {
	t.Run("returns first IP when unused", func(t *testing.T) {
		unreserved := false
		ip, unreserve, err := reserveUnusedIP(
			t.Context(),
			5,
			func(context.Context) (string, func() error, error) {
				return "10.23.133.210", func() error {
					unreserved = true
					return nil
				}, nil
			},
			func(context.Context, string) (bool, string) {
				return false, ""
			},
			func(string, ...interface{}) {},
		)
		require.NoError(t, err)
		assert.Equal(t, "10.23.133.210", ip)
		assert.False(t, unreserved)
		require.NoError(t, unreserve())
		assert.True(t, unreserved)
	})

	t.Run("quarantines collided IPs and returns next unused IP", func(t *testing.T) {
		ips := []string{"10.23.133.202", "10.23.133.203", "10.23.133.204"}
		unreserved := map[string]bool{}
		idx := 0
		var logs []string

		ip, unreserve, err := reserveUnusedIP(
			t.Context(),
			5,
			func(context.Context) (string, func() error, error) {
				candidate := ips[idx]
				idx++
				return candidate, func() error {
					unreserved[candidate] = true
					return nil
				}, nil
			},
			func(_ context.Context, candidate string) (bool, string) {
				if candidate == "10.23.133.204" {
					return false, ""
				}
				return true, "tcp " + candidate + ":6443 connected"
			},
			func(format string, args ...interface{}) {
				logs = append(logs, fmt.Sprintf(format, args...))
			},
		)
		require.NoError(t, err)
		assert.Equal(t, "10.23.133.204", ip)
		assert.Len(t, logs, 2)
		assert.Contains(t, logs[0], "10.23.133.202")
		assert.Contains(t, logs[1], "10.23.133.203")

		// Unreserving the selected IP must leave the collided IPs quarantined.
		require.NoError(t, unreserve())
		assert.False(t, unreserved["10.23.133.202"], "collided IP must remain quarantined")
		assert.False(t, unreserved["10.23.133.203"], "collided IP must remain quarantined")
		assert.True(t, unreserved["10.23.133.204"])
	})

	t.Run("fails after exhausting maxAttempts while keeping collided IPs quarantined", func(t *testing.T) {
		unreservedCount := 0
		_, _, err := reserveUnusedIP(
			t.Context(),
			3,
			func(context.Context) (string, func() error, error) {
				return "10.23.133.202", func() error {
					unreservedCount++
					return nil
				}, nil
			},
			func(context.Context, string) (bool, string) {
				return true, "tcp 10.23.133.202:6443 connected"
			},
			func(string, ...interface{}) {},
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "after 3 attempts")
		assert.Contains(t, err.Error(), "10.23.133.202")
		assert.Zero(t, unreservedCount)
	})

	t.Run("propagates reserve error immediately", func(t *testing.T) {
		probeCalled := false
		_, _, err := reserveUnusedIP(
			t.Context(),
			3,
			func(context.Context) (string, func() error, error) {
				return "", nil, errors.New("prism api unavailable")
			},
			func(context.Context, string) (bool, string) {
				probeCalled = true
				return false, ""
			},
			func(string, ...interface{}) {},
		)
		require.ErrorContains(t, err, "prism api unavailable")
		assert.False(t, probeCalled)
	})
}

func TestUnreserveIPAfterClusterDeleted(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))

	otherCluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "quick-start-other",
			Name:      "quick-start-other-cluster",
		},
	}
	targetCluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "quick-start-abc",
			Name:      "quick-start-target-cluster",
		},
	}

	t.Run("unreserves IP when target Cluster has been deleted", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherCluster).Build()
		unreserved := false

		err := unreserveIPAfterClusterDeleted(
			t.Context(),
			"10.23.133.210",
			func() error {
				unreserved = true
				return nil
			},
			false,
			"quick-start-target-cluster",
			c,
			func(string, ...interface{}) {},
		)
		require.NoError(t, err)
		assert.True(t, unreserved)
	})

	t.Run("keeps IP reserved when target Cluster still exists after timed-out deletion", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(otherCluster, targetCluster).Build()
		unreserved := false
		var logs []string

		err := unreserveIPAfterClusterDeleted(
			t.Context(),
			"10.23.133.202",
			func() error {
				unreserved = true
				return nil
			},
			false,
			"quick-start-target-cluster",
			c,
			func(format string, args ...interface{}) {
				logs = append(logs, fmt.Sprintf(format, args...))
			},
		)
		require.NoError(t, err)
		assert.False(t, unreserved)
		require.Len(t, logs, 1)
		assert.Contains(t, logs[0], "Cluster quick-start-abc/quick-start-target-cluster still exists")
	})

	t.Run("keeps IP reserved when skipCleanup is enabled", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		unreserved := false
		var logs []string

		err := unreserveIPAfterClusterDeleted(
			t.Context(),
			"10.23.133.202",
			func() error {
				unreserved = true
				return nil
			},
			true,
			"quick-start-target-cluster",
			c,
			func(format string, args ...interface{}) {
				logs = append(logs, fmt.Sprintf(format, args...))
			},
		)
		require.NoError(t, err)
		assert.False(t, unreserved)
		require.Len(t, logs, 1)
		assert.Contains(t, logs[0], "skipCleanup is enabled")
	})

	t.Run("keeps IP reserved when listing Clusters fails", func(t *testing.T) {
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithInterceptorFuncs(interceptor.Funcs{
				List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
					return errors.New("apiserver unreachable")
				},
			}).
			Build()
		unreserved := false

		err := unreserveIPAfterClusterDeleted(
			t.Context(),
			"10.23.133.202",
			func() error {
				unreserved = true
				return nil
			},
			false,
			"quick-start-target-cluster",
			c,
			func(string, ...interface{}) {},
		)
		require.NoError(t, err)
		assert.False(t, unreserved)
	})

	t.Run("keeps IP reserved when client is nil or clusterName is empty", func(t *testing.T) {
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		unreserved := false

		require.NoError(t, unreserveIPAfterClusterDeleted(
			t.Context(),
			"10.23.133.202",
			func() error { unreserved = true; return nil },
			false,
			"quick-start-target-cluster",
			nil,
			func(string, ...interface{}) {},
		))
		assert.False(t, unreserved)

		require.NoError(t, unreserveIPAfterClusterDeleted(
			t.Context(),
			"10.23.133.202",
			func() error { unreserved = true; return nil },
			false,
			"",
			c,
			func(string, ...interface{}) {},
		))
		assert.False(t, unreserved)
	})
}
