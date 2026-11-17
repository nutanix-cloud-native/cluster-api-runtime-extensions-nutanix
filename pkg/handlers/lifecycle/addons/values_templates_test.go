// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/yaml"
)

type templateTestCase struct {
	name string
	data any
}

func TestAddonValuesTemplates(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	chartDir := filepath.Join(repoRoot, "charts", "cluster-api-runtime-extensions-nutanix")
	addonsDir := filepath.Join(chartDir, "addons")
	lintConf := filepath.Join(repoRoot, "charts", "lintconf.yaml")

	templatedCases := addonTemplateTestCases()
	// ccm/aws/map-template.yaml and ccm/aws/image-selection.yaml are fragments combined with
	// ccm/aws/values-template.yaml by templates/ccm/aws/manifests/helm-addon-installation.yaml.
	combinedFragmentFiles := map[string]bool{
		"ccm/aws/map-template.yaml":    true,
		"ccm/aws/image-selection.yaml": true,
	}

	err := filepath.WalkDir(addonsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		relPath, err := filepath.Rel(addonsDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if combinedFragmentFiles[relPath] {
			return nil
		}

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(contentBytes)

		if !strings.Contains(content, "{{") {
			t.Run(relPath, func(t *testing.T) {
				t.Parallel()
				assertValidAndLintedYAML(t, lintConf, relPath, content)
			})
			return nil
		}

		cases, ok := templatedCases[relPath]
		require.True(
			t,
			ok,
			"file %s contains Go template directives; add render test cases in values_templates_test.go",
			relPath,
		)

		for _, tc := range cases {
			t.Run(relPath+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				rendered := renderTemplate(t, relPath, content, tc.data)
				assertValidAndLintedYAML(t, lintConf, relPath+" ("+tc.name+")", rendered)
			})
		}

		return nil
	})
	require.NoError(t, err)

	t.Run("ccm/aws/combined-values-template", func(t *testing.T) {
		t.Parallel()

		mapTemplateBytes, err := os.ReadFile(filepath.Join(addonsDir, "ccm", "aws", "map-template.yaml"))
		require.NoError(t, err)
		imageSelectionBytes, err := os.ReadFile(filepath.Join(addonsDir, "ccm", "aws", "image-selection.yaml"))
		require.NoError(t, err)
		valuesTemplateBytes, err := os.ReadFile(filepath.Join(addonsDir, "ccm", "aws", "values-template.yaml"))
		require.NoError(t, err)

		helmValues := map[string]any{
			"Values": map[string]any{
				"hooks": map[string]any{
					"ccm": map[string]any{
						"aws": map[string]any{
							"k8sMinorVersionToCCMVersion": map[string]string{
								"1.35": "v1.35.2",
								"1.36": "v1.36.1",
							},
						},
					},
				},
			},
		}

		renderedMap := renderTemplate(t, "ccm/aws/map-template.yaml", string(mapTemplateBytes), helmValues)
		combinedTemplate := fmt.Sprintf(
			"{{- %s -}}\n%s%s",
			strings.TrimSpace(renderedMap),
			string(imageSelectionBytes),
			string(valuesTemplateBytes),
		)

		caaphInput := map[string]any{
			"Cluster": map[string]any{
				"spec": map[string]any{
					"topology": map[string]any{
						"version": "v1.35.2",
					},
				},
			},
		}
		rendered := renderTemplate(t, "ccm/aws/combined", combinedTemplate, caaphInput)
		assertValidAndLintedYAML(t, lintConf, "ccm/aws/combined", rendered)
	})
}

func TestHelmChartRenderedYAML(t *testing.T) {
	t.Parallel()

	helmBin, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("helm not found in PATH")
	}
	yamllintBin, err := exec.LookPath("yamllint")
	if err != nil {
		t.Skip("yamllint not found in PATH")
	}

	repoRoot := findRepoRoot(t)
	chartDir := filepath.Join(repoRoot, "charts", "cluster-api-runtime-extensions-nutanix")
	lintConf := filepath.Join(repoRoot, "charts", "lintconf.yaml")

	// Pass --api-versions for each infrastructure provider's template Kind so `.Capabilities.APIVersions.Has`
	// guards in templates/clusterclass_{nutanix,aws,eks,docker}.yaml evaluate to true during offline rendering.
	cmd := exec.CommandContext(
		t.Context(),
		helmBin,
		"template",
		chartDir,
		"--api-versions=infrastructure.cluster.x-k8s.io/v1beta1/NutanixClusterTemplate",
		"--api-versions=infrastructure.cluster.x-k8s.io/v1beta2/AWSClusterTemplate",
		"--api-versions=infrastructure.cluster.x-k8s.io/v1beta2/AWSManagedClusterTemplate",
		"--api-versions=infrastructure.cluster.x-k8s.io/v1beta1/DockerClusterTemplate",
	)
	renderedBytes, err := cmd.CombinedOutput()
	require.NoError(t, err, "helm template failed:\n%s", string(renderedBytes))

	lintCmd := exec.CommandContext(t.Context(), yamllintBin, "--strict", "-c", lintConf, "-")
	lintCmd.Stdin = bytes.NewReader(renderedBytes)
	lintOut, err := lintCmd.CombinedOutput()
	require.NoError(t, err, "yamllint failed on rendered Helm chart:\n%s", string(lintOut))
}

func addonTemplateTestCases() map[string][]templateTestCase {
	calicoInput := map[string]any{
		"Cluster": map[string]any{
			"spec": map[string]any{
				"clusterNetwork": map[string]any{
					"pods": map[string]any{
						"cidrBlocks": []string{"192.168.0.0/16", "10.244.0.0/16"},
					},
				},
			},
		},
	}

	clusterAutoscalerCluster := func(provider string) map[string]any {
		return map[string]any{
			"Cluster": &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"caren.nutanix.com/cluster-uuid": "03c19062-493d-4c41-89c3-728624313118",
					},
					Labels: map[string]string{
						"cluster.x-k8s.io/provider": provider,
					},
				},
			},
		}
	}

	return map[string][]templateTestCase{
		"aws-load-balancer-controller/values-template.yaml": {
			{
				name: "eks",
				data: map[string]any{
					"Cluster": map[string]any{
						"metadata": map[string]any{
							"name": "test-cluster",
							"labels": map[string]any{
								"cluster.x-k8s.io/provider": "eks",
							},
						},
					},
					"ControlPlane": map[string]any{
						"spec": map[string]any{
							"eksClusterName": "test-eks-cluster",
						},
					},
				},
			},
			{
				name: "aws",
				data: map[string]any{
					"Cluster": map[string]any{
						"metadata": map[string]any{
							"name": "test-cluster",
							"labels": map[string]any{
								"cluster.x-k8s.io/provider": "aws",
							},
						},
					},
				},
			},
		},
		"ccm/nutanix/values-template.yaml": {
			{
				name: "all-options",
				data: map[string]any{
					"PrismCentralHost":                  "prism-central.example.com",
					"PrismCentralPort":                  uint16(9440),
					"PrismCentralInsecure":              false,
					"PrismCentralAdditionalTrustBundle": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t",
					"IPsToIgnore":                       []string{"1.2.3.4", "5.6.7.8"},
					"ControlPlaneEndpoint": map[string]any{
						"Host": "https://1.2.3.4",
						"Port": int32(6443),
					},
				},
			},
			{
				name: "minimal",
				data: map[string]any{
					"PrismCentralHost":                  "prism-central.example.com",
					"PrismCentralPort":                  uint16(9440),
					"PrismCentralInsecure":              true,
					"PrismCentralAdditionalTrustBundle": "",
					"IPsToIgnore":                       []string(nil),
					"ControlPlaneEndpoint":              nil,
				},
			},
		},
		"cluster-autoscaler/values-template.yaml": {
			{name: "aws", data: clusterAutoscalerCluster("aws")},
			{name: "docker", data: clusterAutoscalerCluster("docker")},
			{name: "nutanix", data: clusterAutoscalerCluster("nutanix")},
			{name: "other", data: clusterAutoscalerCluster("eks")},
		},
		"cni/calico/aws/values-template.yaml": {
			{name: "default", data: calicoInput},
		},
		"cni/calico/docker/values-template.yaml": {
			{name: "default", data: calicoInput},
		},
		"cni/calico/nutanix/values-template.yaml": {
			{name: "default", data: calicoInput},
		},
		"cni/cilium/values-template.yaml": {
			{
				name: "kpr-enabled-eks",
				data: map[string]any{
					"EnableKubeProxyReplacement": true,
					"Provider":                   "eks",
					"ControlPlaneEndpoint": clusterv1.APIEndpoint{
						Host: "https://api.example.com",
						Port: 443,
					},
				},
			},
			{
				name: "kpr-enabled-nutanix",
				data: map[string]any{
					"EnableKubeProxyReplacement": true,
					"Provider":                   "nutanix",
					"ControlPlaneEndpoint": clusterv1.APIEndpoint{
						Host: "192.168.1.100",
						Port: 6443,
					},
				},
			},
			{
				name: "kpr-disabled-eks",
				data: map[string]any{
					"EnableKubeProxyReplacement": false,
					"Provider":                   "eks",
					"ControlPlaneEndpoint": clusterv1.APIEndpoint{
						Host: "https://api.example.com",
						Port: 443,
					},
				},
			},
			{
				name: "kpr-disabled-nutanix",
				data: map[string]any{
					"EnableKubeProxyReplacement": false,
					"Provider":                   "nutanix",
					"ControlPlaneEndpoint": clusterv1.APIEndpoint{
						Host: "192.168.1.100",
						Port: 6443,
					},
				},
			},
		},
		"cni/cilium/preflight-values-template.yaml": {
			{
				name: "kpr-enabled",
				data: map[string]any{
					"EnableKubeProxyReplacement": true,
					"ControlPlaneEndpoint": clusterv1.APIEndpoint{
						Host: "https://api.example.com",
						Port: 6443,
					},
				},
			},
			{
				name: "kpr-disabled",
				data: map[string]any{
					"EnableKubeProxyReplacement": false,
					"ControlPlaneEndpoint": clusterv1.APIEndpoint{
						Host: "https://api.example.com",
						Port: 6443,
					},
				},
			},
		},
		"cni/multus/values-template.yaml": {
			{
				name: "with-readiness-socket",
				data: map[string]any{
					"ReadinessSocketPath": "/var/run/cilium/cilium.sock",
				},
			},
			{
				name: "without-readiness-socket",
				data: map[string]any{
					"ReadinessSocketPath": "",
				},
			},
		},
		"cni/nutanix-flow/values-template.yaml": {
			{
				name: "with-image-pull-secret",
				data: map[string]any{
					"ControlPlaneEndpoint": clusterv1.APIEndpoint{
						Host: "192.168.1.100",
						Port: 6443,
					},
					"PodCIDR":             "10.244.0.0/16",
					"ServiceCIDR":         "10.96.0.0/12",
					"ImagePullSecretName": "flow-cni-image-pull-secret",
				},
			},
			{
				name: "without-image-pull-secret",
				data: map[string]any{
					"ControlPlaneEndpoint": clusterv1.APIEndpoint{
						Host: "192.168.1.100",
						Port: 6443,
					},
					"PodCIDR":             "10.244.0.0/16",
					"ServiceCIDR":         "10.96.0.0/12",
					"ImagePullSecretName": "",
				},
			},
		},
		"konnector-agent/values-template.yaml": {
			{
				name: "all-options",
				data: map[string]any{
					"AgentName":                            "konnector-agent",
					"PrismCentralHost":                     "prism-central.example.com",
					"PrismCentralPort":                     uint16(9440),
					"PrismCentralInsecure":                 false,
					"PrismCentralAdditionalTrustBundle":    "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t",
					"CreateAdditionalTrustBundleConfigMap": true,
					"AdditionalTrustBundleConfigMapName":   "trust-bundle-cm",
					"ClusterName":                          "test-cluster",
					"CategoryMappings":                     "Environment=Production,Department=Engineering",
					"PrismCredentialsSecretName":           "konnector-agent-creds",
					"EnableKubeconfigUpload":               true,
					"ControlPlaneEndpoint":                 "https://1.2.3.4:6443",
				},
			},
			{
				name: "minimal",
				data: map[string]any{
					"AgentName":                            "konnector-agent",
					"PrismCentralHost":                     "prism-central.example.com",
					"PrismCentralPort":                     uint16(9440),
					"PrismCentralInsecure":                 true,
					"PrismCentralAdditionalTrustBundle":    "",
					"CreateAdditionalTrustBundleConfigMap": false,
					"AdditionalTrustBundleConfigMapName":   "",
					"ClusterName":                          "test-cluster",
					"CategoryMappings":                     "",
					"PrismCredentialsSecretName":           "konnector-agent-creds",
					"EnableKubeconfigUpload":               false,
					"ControlPlaneEndpoint":                 "",
				},
			},
		},
		"registry/cncf-distribution/values-template.yaml": {
			{
				name: "default",
				data: map[string]any{
					"Replicas":      int32(1),
					"ServiceIP":     "10.96.0.10",
					"TLSSecretName": "registry-tls",
				},
			},
		},
		"registry-syncer/values-template.yaml": {
			{
				name: "default",
				data: map[string]any{
					"CusterName":                                  "test-cluster",
					"KubernetesVersion":                           "v1.35.0",
					"SourceRegistryAddress":                       "10.96.0.10:5000",
					"DestinationRegistryAnyPodName":               "cncf-distribution-registry-0",
					"DestinationRegistryHeadlessServiceNamespace": "registry-system",
					"DestinationRegistryHeadlessServiceName":      "cncf-distribution-registry-headless",
					"DestinationRegistryHeadlessServicePort":      int32(5000),
					"RegistryCASecretName":                        "registry-ca",
				},
			},
		},
	}
}

func renderTemplate(t *testing.T, name, text string, data any) string {
	t.Helper()

	funcMap := template.FuncMap{
		"trimPrefix": strings.TrimPrefix,
		"joinQuoted": func(items []string) string {
			quoted := make([]string, len(items))
			for i, item := range items {
				quoted[i] = fmt.Sprintf("%q", item)
			}
			return strings.Join(quoted, ", ")
		},
		"dict": func(values ...any) map[string]any {
			m := make(map[string]any, len(values)/2)
			for i := 0; i+1 < len(values); i += 2 {
				m[fmt.Sprint(values[i])] = values[i+1]
			}
			return m
		},
		"get": func(m map[string]any, key string) any {
			if val, ok := m[key]; ok {
				return val
			}
			return ""
		},
		"semver": semver.ParseTolerant,
	}

	tmpl, err := template.New(name).Funcs(funcMap).Parse(text)
	require.NoError(t, err, "failed to parse template %s", name)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	require.NoError(t, err, "failed to execute template %s", name)

	rendered := buf.String()
	assert.NotContains(t, rendered, "<no value>", "template %s rendered <no value>", name)
	assert.NotContains(t, rendered, "{{", "template %s still contains unrendered template directives", name)

	return rendered
}

func assertValidAndLintedYAML(t *testing.T, lintConf, name, content string) {
	t.Helper()

	var parsed map[string]any
	err := yaml.Unmarshal([]byte(content), &parsed)
	require.NoError(t, err, "rendered YAML for %s is invalid:\n%s", name, content)
	if strings.TrimSpace(content) != "" {
		require.NotEmpty(t, parsed, "rendered YAML for %s is empty", name)
	}

	yamllintBin, err := exec.LookPath("yamllint")
	if err != nil {
		t.Skip("yamllint not found in PATH")
	}

	cmd := exec.CommandContext(t.Context(), yamllintBin, "--strict", "-c", lintConf, "-")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yamllint failed for %s:\n%s\nRendered YAML:\n%s", name, string(out), content)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{ .Dir }}")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to get module root dir: %s", string(out))
	dir, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return dir
}
