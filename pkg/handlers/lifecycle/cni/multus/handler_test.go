// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/yaml"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	testClusterUUID      = "03c19062-493d-4c41-89c3-728624313118"
	testClusterName      = "test-cluster"
	testClusterNamespace = "cluster-ns"
	testDefaultsNS       = "caren-system"
	testHelmAddonsCM     = "default-helm-addons-config"
)

func TestDefaultValuesTemplate_NoDeadVolumes(t *testing.T) {
	t.Parallel()

	modRootCmd := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{ .Dir }}")
	out, err := modRootCmd.CombinedOutput()
	require.NoError(t, err)
	repoRoot, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")

	templatePath := filepath.Join(
		repoRoot,
		"charts",
		"cluster-api-runtime-extensions-nutanix",
		"addons",
		"cni",
		"multus",
		"values-template.yaml",
	)
	rawBytes, err := os.ReadFile(templatePath)
	require.NoError(t, err)

	templater := templateValuesFunc(v1alpha1.CNI{Provider: v1alpha1.CNIProviderCilium})
	rendered, err := templater(nil, string(rawBytes))
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(rendered), &parsed))
	assert.NotContains(t, parsed, "volumes")
	assert.NotContains(t, parsed, "volumeMounts")

	daemonConfig, ok := parsed["daemonConfig"].(map[string]any)
	require.True(t, ok, "expected daemonConfig map in rendered values")
	assert.Equal(t, "/run/cilium/cilium.sock", daemonConfig["readinessIndicatorFile"])
}

func TestMultusHandler_OptInAcrossProvidersAndCNIs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name               string
		provider           string
		cniProvider        string
		expectedSocketPath string
	}{
		{
			name:               "docker with Cilium",
			provider:           "docker",
			cniProvider:        v1alpha1.CNIProviderCilium,
			expectedSocketPath: "/run/cilium/cilium.sock",
		},
		{
			name:               "aws with Calico",
			provider:           "aws",
			cniProvider:        v1alpha1.CNIProviderCalico,
			expectedSocketPath: "/var/run/calico/cni-server.sock",
		},
		{
			name:               "eks with Cilium",
			provider:           "eks",
			cniProvider:        v1alpha1.CNIProviderCilium,
			expectedSocketPath: "/run/cilium/cilium.sock",
		},
		{
			name:               "nutanix with Flow",
			provider:           "nutanix",
			cniProvider:        v1alpha1.CNIProviderFlow,
			expectedSocketPath: "/var/run/ovn-kubernetes/cni/ovn-cni-server.sock",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cluster := newTestCluster(tc.provider, &v1alpha1.CNI{
				Provider: tc.cniProvider,
				GenericCNI: v1alpha1.GenericCNI{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					Multus: &v1alpha1.Multus{
						Strategy: v1alpha1.AddonStrategyHelmAddon,
					},
				},
			})

			cl, handler := newTestHandler(t, cluster)
			resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
			handler.AfterControlPlaneInitialized(
				t.Context(),
				&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
				resp,
			)

			require.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)

			hcp := &caaphv1.HelmChartProxy{}
			require.NoError(t, cl.Get(
				t.Context(),
				ctrlclient.ObjectKey{
					Namespace: testClusterNamespace,
					Name:      "multus-" + testClusterUUID,
				},
				hcp,
			))
			assert.Equal(t, "0.1.1", hcp.Spec.Version)
			assert.Equal(t, "multus", hcp.Spec.ReleaseName)
			assert.Equal(t, metav1.NamespaceSystem, hcp.Spec.ReleaseNamespace)

			var values map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(hcp.Spec.ValuesTemplate), &values))
			daemonConfig, ok := values["daemonConfig"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.expectedSocketPath, daemonConfig["readinessIndicatorFile"])
		})
	}
}

func TestMultusHandler_SkipsWhenOptedOutAndNoExistingRelease(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"docker", "aws", "eks", "nutanix"} {
		t.Run(provider, func(t *testing.T) {
			t.Parallel()

			cluster := newTestCluster(provider, &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				GenericCNI: v1alpha1.GenericCNI{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
				},
			})

			cl, handler := newTestHandler(t, cluster)
			resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
			handler.AfterControlPlaneInitialized(
				t.Context(),
				&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
				resp,
			)

			require.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)

			hcp := &caaphv1.HelmChartProxy{}
			err := cl.Get(
				t.Context(),
				ctrlclient.ObjectKey{
					Namespace: testClusterNamespace,
					Name:      "multus-" + testClusterUUID,
				},
				hcp,
			)
			assert.True(t, apierrors.IsNotFound(err), "expected no HelmChartProxy to be created")
		})
	}
}

func TestMultusHandler_ReconcilesPreExistingMultusOnUpgradeWithoutMutatingCluster(t *testing.T) {
	t.Parallel()

	cluster := newTestCluster("nutanix", &v1alpha1.CNI{
		Provider: v1alpha1.CNIProviderCilium,
		GenericCNI: v1alpha1.GenericCNI{
			Strategy: v1alpha1.AddonStrategyHelmAddon,
			// Multus intentionally nil to simulate a pre-existing cluster created before opt-in Multus.
		},
	})
	originalClusterVars := append([]clusterv1.ClusterVariable(nil), cluster.Spec.Topology.Variables...)

	existingHCP := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testClusterNamespace,
			Name:      "multus-" + testClusterUUID,
		},
		Spec: caaphv1.HelmChartProxySpec{
			ChartName:        "multus",
			ReleaseName:      "multus",
			ReleaseNamespace: metav1.NamespaceSystem,
			Version:          "0.1.0",
		},
	}

	cl, handler := newTestHandler(t, cluster, existingHCP)
	resp := &runtimehooksv1.BeforeClusterUpgradeResponse{}
	handler.BeforeClusterUpgrade(
		t.Context(),
		&runtimehooksv1.BeforeClusterUpgradeRequest{Cluster: *cluster},
		resp,
	)

	require.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)

	updatedHCP := &caaphv1.HelmChartProxy{}
	require.NoError(t, cl.Get(
		t.Context(),
		ctrlclient.ObjectKey{
			Namespace: testClusterNamespace,
			Name:      "multus-" + testClusterUUID,
		},
		updatedHCP,
	))
	assert.Equal(t, "0.1.1", updatedHCP.Spec.Version)
	assert.Contains(t, updatedHCP.Spec.ValuesTemplate, "/run/cilium/cilium.sock")

	storedCluster := &clusterv1.Cluster{}
	require.NoError(t, cl.Get(t.Context(), ctrlclient.ObjectKeyFromObject(cluster), storedCluster))
	assert.Equal(t, originalClusterVars, storedCluster.Spec.Topology.Variables)
}

func TestMultusHandler_CustomValuesSourceRef(t *testing.T) {
	t.Parallel()

	t.Run("injects readinessIndicatorFile when omitted and sets Cluster OwnerReference", func(t *testing.T) {
		t.Parallel()

		customCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testClusterNamespace,
				Name:      "custom-multus-values",
			},
			Data: map[string]string{
				"values.yaml": "daemonConfig:\n  logLevel: debug\nnodeSelector:\n  kubernetes.io/os: linux\n",
			},
		}

		cluster := newTestCluster("nutanix", &v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderCalico,
			GenericCNI: v1alpha1.GenericCNI{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
				Multus: &v1alpha1.Multus{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					AddonConfig: v1alpha1.AddonConfig{
						Values: &v1alpha1.AddonValues{
							SourceRef: &v1alpha1.ValuesReference{
								Kind: "ConfigMap",
								Name: customCM.Name,
							},
						},
					},
				},
			},
		})

		cl, handler := newTestHandler(t, cluster, customCM)
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handler.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
			resp,
		)

		require.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)

		// Verify OwnerReference set on custom ConfigMap.
		updatedCM := &corev1.ConfigMap{}
		require.NoError(t, cl.Get(t.Context(), ctrlclient.ObjectKeyFromObject(customCM), updatedCM))
		require.Len(t, updatedCM.OwnerReferences, 1)
		assert.Equal(t, cluster.Name, updatedCM.OwnerReferences[0].Name)
		assert.Equal(t, "Cluster", updatedCM.OwnerReferences[0].Kind)

		// Verify rendered HelmChartProxy values keep custom fields and inject readinessIndicatorFile.
		hcp := &caaphv1.HelmChartProxy{}
		require.NoError(t, cl.Get(
			t.Context(),
			ctrlclient.ObjectKey{
				Namespace: testClusterNamespace,
				Name:      "multus-" + testClusterUUID,
			},
			hcp,
		))

		var values map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(hcp.Spec.ValuesTemplate), &values))
		daemonConfig, ok := values["daemonConfig"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "debug", daemonConfig["logLevel"])
		assert.Equal(t, "/var/run/calico/cni-server.sock", daemonConfig["readinessIndicatorFile"])

		nodeSelector, ok := values["nodeSelector"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "linux", nodeSelector["kubernetes.io/os"])
	})

	t.Run("injects readinessIndicatorFile when empty string", func(t *testing.T) {
		t.Parallel()

		customCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testClusterNamespace,
				Name:      "custom-multus-empty-socket",
			},
			Data: map[string]string{
				"values.yaml": "daemonConfig:\n  readinessIndicatorFile: \"\"\n",
			},
		}

		cluster := newTestCluster("docker", &v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
			GenericCNI: v1alpha1.GenericCNI{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
				Multus: &v1alpha1.Multus{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					AddonConfig: v1alpha1.AddonConfig{
						Values: &v1alpha1.AddonValues{
							SourceRef: &v1alpha1.ValuesReference{
								Kind: "ConfigMap",
								Name: customCM.Name,
							},
						},
					},
				},
			},
		})

		cl, handler := newTestHandler(t, cluster, customCM)
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handler.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
			resp,
		)

		require.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)

		hcp := &caaphv1.HelmChartProxy{}
		require.NoError(t, cl.Get(
			t.Context(),
			ctrlclient.ObjectKey{
				Namespace: testClusterNamespace,
				Name:      "multus-" + testClusterUUID,
			},
			hcp,
		))

		var values map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(hcp.Spec.ValuesTemplate), &values))
		daemonConfig, ok := values["daemonConfig"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "/run/cilium/cilium.sock", daemonConfig["readinessIndicatorFile"])
	})

	t.Run("injects readinessIndicatorFile when values.yaml is empty or comment-only", func(t *testing.T) {
		t.Parallel()

		customCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testClusterNamespace,
				Name:      "custom-multus-comment-only",
			},
			Data: map[string]string{
				"values.yaml": "# comment only\n",
			},
		}

		cluster := newTestCluster("docker", &v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
			GenericCNI: v1alpha1.GenericCNI{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
				Multus: &v1alpha1.Multus{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					AddonConfig: v1alpha1.AddonConfig{
						Values: &v1alpha1.AddonValues{
							SourceRef: &v1alpha1.ValuesReference{
								Kind: "ConfigMap",
								Name: customCM.Name,
							},
						},
					},
				},
			},
		})

		cl, handler := newTestHandler(t, cluster, customCM)
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handler.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
			resp,
		)

		require.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)

		hcp := &caaphv1.HelmChartProxy{}
		require.NoError(t, cl.Get(
			t.Context(),
			ctrlclient.ObjectKey{
				Namespace: testClusterNamespace,
				Name:      "multus-" + testClusterUUID,
			},
			hcp,
		))

		var values map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(hcp.Spec.ValuesTemplate), &values))
		daemonConfig, ok := values["daemonConfig"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "/run/cilium/cilium.sock", daemonConfig["readinessIndicatorFile"])
	})

	t.Run("preserves explicit non-empty readinessIndicatorFile verbatim", func(t *testing.T) {
		t.Parallel()

		explicitYAML := "# user comment\ndaemonConfig:\n  readinessIndicatorFile: /custom/cni.sock\n"
		customCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testClusterNamespace,
				Name:      "custom-multus-explicit-socket",
			},
			Data: map[string]string{
				"values.yaml": explicitYAML,
			},
		}

		cluster := newTestCluster("aws", &v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
			GenericCNI: v1alpha1.GenericCNI{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
				Multus: &v1alpha1.Multus{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					AddonConfig: v1alpha1.AddonConfig{
						Values: &v1alpha1.AddonValues{
							SourceRef: &v1alpha1.ValuesReference{
								Kind: "ConfigMap",
								Name: customCM.Name,
							},
						},
					},
				},
			},
		})

		cl, handler := newTestHandler(t, cluster, customCM)
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handler.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
			resp,
		)

		require.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status, resp.Message)

		hcp := &caaphv1.HelmChartProxy{}
		require.NoError(t, cl.Get(
			t.Context(),
			ctrlclient.ObjectKey{
				Namespace: testClusterNamespace,
				Name:      "multus-" + testClusterUUID,
			},
			hcp,
		))
		assert.Equal(t, explicitYAML, hcp.Spec.ValuesTemplate)
	})

	t.Run("fails when referenced ConfigMap does not exist", func(t *testing.T) {
		t.Parallel()

		cluster := newTestCluster("nutanix", &v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
			GenericCNI: v1alpha1.GenericCNI{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
				Multus: &v1alpha1.Multus{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					AddonConfig: v1alpha1.AddonConfig{
						Values: &v1alpha1.AddonValues{
							SourceRef: &v1alpha1.ValuesReference{
								Kind: "ConfigMap",
								Name: "missing-multus-values",
							},
						},
					},
				},
			},
		})

		_, handler := newTestHandler(t, cluster)
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handler.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
			resp,
		)

		assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
		assert.Contains(t, resp.Message, "missing-multus-values")
	})

	t.Run("fails when daemonConfig in custom values is not a map", func(t *testing.T) {
		t.Parallel()

		customCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testClusterNamespace,
				Name:      "invalid-daemonconfig",
			},
			Data: map[string]string{
				"values.yaml": "daemonConfig: invalid\n",
			},
		}

		cluster := newTestCluster("nutanix", &v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
			GenericCNI: v1alpha1.GenericCNI{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
				Multus: &v1alpha1.Multus{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					AddonConfig: v1alpha1.AddonConfig{
						Values: &v1alpha1.AddonValues{
							SourceRef: &v1alpha1.ValuesReference{
								Kind: "ConfigMap",
								Name: customCM.Name,
							},
						},
					},
				},
			},
		})

		_, handler := newTestHandler(t, cluster, customCM)
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handler.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
			resp,
		)

		assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
		assert.Contains(t, resp.Message, "daemonConfig")
	})
}

func TestMultusHandler_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("skips when addons.cni is not set", func(t *testing.T) {
		t.Parallel()

		cluster := newTestCluster("nutanix", nil)
		cl, handler := newTestHandler(t, cluster)
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handler.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
			resp,
		)

		assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
		hcp := &caaphv1.HelmChartProxy{}
		err := cl.Get(
			t.Context(),
			ctrlclient.ObjectKey{
				Namespace: testClusterNamespace,
				Name:      "multus-" + testClusterUUID,
			},
			hcp,
		)
		assert.True(t, apierrors.IsNotFound(err))
	})

	t.Run("fails when strategy is empty or unsupported", func(t *testing.T) {
		t.Parallel()

		for _, strategy := range []v1alpha1.AddonStrategy{"", v1alpha1.AddonStrategyClusterResourceSet} {
			cluster := newTestCluster("nutanix", &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				GenericCNI: v1alpha1.GenericCNI{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					Multus: &v1alpha1.Multus{
						Strategy: strategy,
					},
				},
			})

			_, handler := newTestHandler(t, cluster)
			resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
			handler.AfterControlPlaneInitialized(
				t.Context(),
				&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster},
				resp,
			)

			assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
		}
	})

	t.Run("handles missing cluster-uuid annotation", func(t *testing.T) {
		t.Parallel()

		// When opted out, missing cluster-uuid skips cleanly without error.
		optedOutCluster := newTestCluster("nutanix", &v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
			GenericCNI: v1alpha1.GenericCNI{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
			},
		})
		delete(optedOutCluster.Annotations, v1alpha1.ClusterUUIDAnnotationKey)

		_, handler := newTestHandler(t, optedOutCluster)
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handler.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *optedOutCluster},
			resp,
		)
		assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)

		// When opted in, missing cluster-uuid fails cleanly via HelmAddonApplier.
		optedInCluster := newTestCluster("nutanix", &v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
			GenericCNI: v1alpha1.GenericCNI{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
				Multus: &v1alpha1.Multus{
					Strategy: v1alpha1.AddonStrategyHelmAddon,
				},
			},
		})
		delete(optedInCluster.Annotations, v1alpha1.ClusterUUIDAnnotationKey)

		_, handlerOptIn := newTestHandler(t, optedInCluster)
		respOptIn := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		handlerOptIn.AfterControlPlaneInitialized(
			t.Context(),
			&runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *optedInCluster},
			respOptIn,
		)
		assert.Equal(t, runtimehooksv1.ResponseStatusFailure, respOptIn.Status)
		assert.Contains(t, respOptIn.Message, v1alpha1.ClusterUUIDAnnotationKey)
	})
}

func newTestCluster(provider string, cniVar *v1alpha1.CNI) *clusterv1.Cluster {
	var vars []clusterv1.ClusterVariable
	if cniVar != nil {
		cv, err := apivariables.MarshalToClusterVariable(
			v1alpha1.ClusterConfigVariableName,
			&apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CNI: cniVar,
				},
			},
		)
		if err != nil {
			panic(err)
		}
		vars = append(vars, *cv)
	}

	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testClusterNamespace,
			UID:       types.UID("cluster-uid-1234"),
			Labels: map[string]string{
				clusterv1.ProviderNameLabel: provider,
			},
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: testClusterUUID,
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef:  clusterv1.ClusterClassRef{Name: "dummy-class"},
				Variables: vars,
			},
		},
	}
}

func newTestHandler(t *testing.T, objs ...ctrlclient.Object) (ctrlclient.Client, *MultusHandler) {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(caaphv1.AddToScheme(scheme))

	helmAddonsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testDefaultsNS,
			Name:      testHelmAddonsCM,
		},
		Data: map[string]string{
			string(config.Multus): "ChartName: multus\nChartVersion: 0.1.1\nRepositoryURL: https://mesosphere.github.io/charts/stable/\n",
		},
	}

	defaultValuesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testDefaultsNS,
			Name:      "default-multus-values-template",
		},
		Data: map[string]string{
			"values.yaml": "daemonConfig:\n  readinessIndicatorFile: \"{{ .ReadinessSocketPath }}\"\nresources:\n  limits:\n    memory: null\n",
		},
	}

	allObjs := append([]ctrlclient.Object{helmAddonsCM, defaultValuesCM}, objs...)

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&caaphv1.HelmChartProxy{}).
		WithObjects(allObjs...).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(
				ctx context.Context,
				c ctrlclient.WithWatch,
				obj ctrlclient.Object,
				patch ctrlclient.Patch,
				opts ...ctrlclient.PatchOption,
			) error {
				if err := c.Patch(ctx, obj, patch, opts...); err != nil {
					return err
				}
				hcp, ok := obj.(*caaphv1.HelmChartProxy)
				if !ok {
					return nil
				}
				hcp.Status.ObservedGeneration = hcp.Generation
				apimeta.SetStatusCondition(&hcp.Status.Conditions, metav1.Condition{
					Type:   clusterv1.ReadyCondition,
					Status: metav1.ConditionTrue,
					Reason: "Ready",
				})
				return c.Status().Update(ctx, hcp)
			},
		}).
		Build()

	globalOpts := options.NewGlobalOptions()
	pflags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	globalOpts.AddFlags(pflags)

	cfg := NewMultusConfig(globalOpts)
	cfg.AddFlags("cni.multus", pflags)
	require.NoError(t, pflags.Parse([]string{
		"--defaults-namespace=" + testDefaultsNS,
		"--helm-addons-configmap=" + testHelmAddonsCM,
	}))

	getter := config.NewHelmChartGetterFromConfigMap(
		globalOpts.HelmAddonsConfigMapName(),
		globalOpts.DefaultsNamespace(),
		cl,
	)

	return cl, New(cl, cfg, getter)
}
