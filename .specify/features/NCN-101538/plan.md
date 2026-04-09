<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# NCN-101538: Pod Security Admission Configuration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable users to configure the Kubernetes PodSecurity admission plugin via a new `podSecurityAdmission` CAREN variable on `KubeadmClusterConfigSpec`.

**Architecture:** A new API type `PodSecurityAdmission` is added to `KubeadmClusterConfigSpec`. A reusable `admissionconfiguration` package handles coordination with the API server's `AdmissionConfiguration` file and extra args. A thin `podsecurityadmission` handler reads the variable, generates a `PodSecurityConfiguration` YAML, and delegates to the shared package.

**Tech Stack:** Go, kubebuilder markers, CAPI runtime hooks (v1beta2), Ginkgo/Gomega tests, controller-gen for CRD generation.

---

## File Structure

### New files

| File | Responsibility |
|------|---------------|
| `api/v1alpha1/pod_security_admission_types.go` | `PodSecurityAdmission`, `PodSecurityExemptions`, `PodSecurityStandard` types with kubebuilder markers |
| `pkg/handlers/generic/mutation/kubeadm/admissionconfiguration/admission_configuration.go` | Shared `AddPlugin()` function for managing `AdmissionConfiguration` |
| `pkg/handlers/generic/mutation/kubeadm/admissionconfiguration/admission_configuration_test.go` | Unit tests for the shared package |
| `pkg/handlers/generic/mutation/kubeadm/podsecurityadmission/inject.go` | PSA mutation handler |
| `pkg/handlers/generic/mutation/kubeadm/podsecurityadmission/inject_test.go` | PSA handler tests |
| `docs/content/customization/kubeadm/pod-security-admission.md` | User documentation |

### Modified files

| File | Change |
|------|--------|
| `api/v1alpha1/clusterconfig_types.go` | Add `PodSecurityAdmission *PodSecurityAdmission` field to `KubeadmClusterConfigSpec` |
| `pkg/handlers/generic/mutation/handlers.go` | Register `podsecurityadmission.NewPatch()` in `MetaMutators()` |

### Generated files (via `make go-generate`)

| File | Change |
|------|--------|
| `api/v1alpha1/zz_generated.deepcopy.go` | DeepCopy methods for new types |
| `api/v1alpha1/crds/*.yaml` | Updated CRD schemas including `podSecurityAdmission` |

---

## Task 1: Add API types

**Files:**
- Create: `api/v1alpha1/pod_security_admission_types.go`
- Modify: `api/v1alpha1/clusterconfig_types.go:261-287`

- [ ] **Step 1: Create the PSA types file**

Create `api/v1alpha1/pod_security_admission_types.go`:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// PodSecurityStandard defines the Pod Security Standard levels.
// +kubebuilder:validation:Enum=privileged;baseline;restricted
type PodSecurityStandard string

const (
	PodSecurityStandardPrivileged PodSecurityStandard = "privileged"
	PodSecurityStandardBaseline   PodSecurityStandard = "baseline"
	PodSecurityStandardRestricted PodSecurityStandard = "restricted"
)

// PodSecurityAdmission configures the PodSecurity admission plugin with cluster-wide defaults.
// When not specified on KubeadmClusterConfigSpec, no PodSecurity admission configuration is
// applied (no-op for existing clusters).
type PodSecurityAdmission struct {
	// Enforce sets the level for the enforce mode.
	// Pods that violate this level will be rejected.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=privileged
	// +kubebuilder:validation:Enum=privileged;baseline;restricted
	Enforce PodSecurityStandard `json:"enforce,omitempty"`

	// Audit sets the level for the audit mode.
	// Violations are recorded in the API server audit log.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=privileged
	// +kubebuilder:validation:Enum=privileged;baseline;restricted
	Audit PodSecurityStandard `json:"audit,omitempty"`

	// Warn sets the level for the warn mode.
	// Violations trigger a user-facing warning.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=privileged
	// +kubebuilder:validation:Enum=privileged;baseline;restricted
	Warn PodSecurityStandard `json:"warn,omitempty"`

	// Exemptions defines the exemptions from pod security enforcement.
	// +kubebuilder:validation:Optional
	Exemptions PodSecurityExemptions `json:"exemptions,omitempty"`
}

// PodSecurityExemptions defines resources exempt from pod security enforcement.
type PodSecurityExemptions struct {
	// Namespaces that are exempt from pod security enforcement.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={"kube-system"}
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=63
	Namespaces []string `json:"namespaces,omitempty"`

	// Usernames that are exempt from pod security enforcement.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=256
	Usernames []string `json:"usernames,omitempty"`

	// RuntimeClassNames that are exempt from pod security enforcement.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=63
	RuntimeClassNames []string `json:"runtimeClassNames,omitempty"`
}
```

- [ ] **Step 2: Add field to KubeadmClusterConfigSpec**

In `api/v1alpha1/clusterconfig_types.go`, add the following field to `KubeadmClusterConfigSpec` after the `MaxParallelImagePullsPerNode` field (before the closing brace at line 287):

```go
	// PodSecurityAdmission configures the PodSecurity admission plugin
	// with cluster-wide defaults.
	// When not specified, no PodSecurity admission configuration is applied.
	// +kubebuilder:validation:Optional
	PodSecurityAdmission *PodSecurityAdmission `json:"podSecurityAdmission,omitempty"`
```

- [ ] **Step 3: Run code generation**

Run: `make go-generate`

This regenerates:
- `api/v1alpha1/zz_generated.deepcopy.go` (DeepCopy for new types)
- `api/v1alpha1/crds/*.yaml` (CRD schemas with new field)

Expected: Command succeeds. New types appear in deepcopy file. CRD YAML files for `kubeadmclusterconfigs`, `awsclusterconfigs`, `dockerclusterconfigs`, and `nutanixclusterconfigs` include `podSecurityAdmission` in their schema.

- [ ] **Step 4: Verify CRD schema**

Run: `grep -A 5 'podSecurityAdmission' api/v1alpha1/crds/caren.nutanix.com_kubeadmclusterconfigs.yaml | head -20`

Expected: The field appears with `enforce`, `audit`, `warn` (each with `default: privileged` and enum), and `exemptions` with `namespaces` defaulting to `["kube-system"]`.

- [ ] **Step 5: Commit**

```bash
git add api/v1alpha1/pod_security_admission_types.go api/v1alpha1/clusterconfig_types.go api/v1alpha1/zz_generated.deepcopy.go api/v1alpha1/crds/
git commit -m "feat: [NCN-101538] Add PodSecurityAdmission API types

Add PodSecurityAdmission, PodSecurityExemptions, and PodSecurityStandard
types to KubeadmClusterConfigSpec for configuring the PodSecurity
admission plugin via CAREN variables."
```

---

## Task 2: Create the shared admissionconfiguration package

**Files:**
- Create: `pkg/handlers/generic/mutation/kubeadm/admissionconfiguration/admission_configuration.go`

- [ ] **Step 1: Write the test file first**

Create `pkg/handlers/generic/mutation/kubeadm/admissionconfiguration/admission_configuration_test.go`:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package admissionconfiguration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
)

func TestAddPlugin_NoExistingAdmissionConfig(t *testing.T) {
	kcp := &controlplanev1.KubeadmControlPlaneTemplate{}

	err := AddPlugin(kcp, Plugin{
		Name:              "PodSecurity",
		ConfigFilePath:    "/etc/kubernetes/pod-security-admission.yaml",
		ConfigFileContent: "test-content",
	})
	require.NoError(t, err)

	spec := &kcp.Spec.Template.Spec.KubeadmConfigSpec

	// Should set admission-control-config-file extra arg
	found := false
	for _, arg := range spec.ClusterConfiguration.APIServer.ExtraArgs {
		if arg.Name == "admission-control-config-file" {
			found = true
			assert.Equal(t, defaultAdmissionConfigPath, *arg.Value)
		}
	}
	assert.True(t, found, "admission-control-config-file arg not found")

	// Should add admission config file
	assertFileExists(t, spec.Files, defaultAdmissionConfigPath)
	// Should add plugin config file
	assertFileExists(t, spec.Files, "/etc/kubernetes/pod-security-admission.yaml")

	// Should add volume mounts
	assertVolumeMountExists(t, spec.ClusterConfiguration.APIServer.ExtraVolumes, defaultAdmissionConfigPath)
	assertVolumeMountExists(t, spec.ClusterConfiguration.APIServer.ExtraVolumes, "/etc/kubernetes/pod-security-admission.yaml")

	// Should add enable-admission-plugins
	assertAdmissionPluginEnabled(t, spec.ClusterConfiguration.APIServer.ExtraArgs, "PodSecurity")
}

func TestAddPlugin_ExistingAdmissionConfigFileAndArg(t *testing.T) {
	existingAdmissionConfig := `apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: EventRateLimit
  path: /etc/kubernetes/eventratelimit-config.yaml
`
	kcp := &controlplanev1.KubeadmControlPlaneTemplate{}
	spec := &kcp.Spec.Template.Spec.KubeadmConfigSpec
	spec.ClusterConfiguration.APIServer.ExtraArgs = []bootstrapv1.Arg{
		{Name: "admission-control-config-file", Value: ptr.To("/etc/kubernetes/admission.yaml")},
		{Name: "enable-admission-plugins", Value: ptr.To("EventRateLimit,NodeRestriction")},
	}
	spec.Files = []bootstrapv1.File{
		{Path: "/etc/kubernetes/admission.yaml", Content: existingAdmissionConfig},
	}

	err := AddPlugin(kcp, Plugin{
		Name:              "PodSecurity",
		ConfigFilePath:    "/etc/kubernetes/pod-security-admission.yaml",
		ConfigFileContent: "test-content",
	})
	require.NoError(t, err)

	// Should preserve existing plugins in AdmissionConfiguration
	admissionFile := findFile(spec.Files, "/etc/kubernetes/admission.yaml")
	require.NotNil(t, admissionFile)
	assert.Contains(t, admissionFile.Content, "EventRateLimit")
	assert.Contains(t, admissionFile.Content, "PodSecurity")

	// Should add plugin config file
	assertFileExists(t, spec.Files, "/etc/kubernetes/pod-security-admission.yaml")

	// Should append to enable-admission-plugins without duplicating
	assertAdmissionPluginEnabled(t, spec.ClusterConfiguration.APIServer.ExtraArgs, "PodSecurity")
	assertAdmissionPluginEnabled(t, spec.ClusterConfiguration.APIServer.ExtraArgs, "EventRateLimit")
}

func TestAddPlugin_ExistingArgButNoFile(t *testing.T) {
	customPath := "/custom/path/admission.yaml"
	kcp := &controlplanev1.KubeadmControlPlaneTemplate{}
	spec := &kcp.Spec.Template.Spec.KubeadmConfigSpec
	spec.ClusterConfiguration.APIServer.ExtraArgs = []bootstrapv1.Arg{
		{Name: "admission-control-config-file", Value: ptr.To(customPath)},
	}

	err := AddPlugin(kcp, Plugin{
		Name:              "PodSecurity",
		ConfigFilePath:    "/etc/kubernetes/pod-security-admission.yaml",
		ConfigFileContent: "test-content",
	})
	require.NoError(t, err)

	// Should create file at the custom path
	assertFileExists(t, spec.Files, customPath)
	// Should NOT change the extra arg path
	for _, arg := range spec.ClusterConfiguration.APIServer.ExtraArgs {
		if arg.Name == "admission-control-config-file" {
			assert.Equal(t, customPath, *arg.Value)
		}
	}
}

func TestAddPlugin_PluginAlreadyPresent(t *testing.T) {
	existingAdmissionConfig := `apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: PodSecurity
  path: /etc/kubernetes/pod-security-admission.yaml
`
	kcp := &controlplanev1.KubeadmControlPlaneTemplate{}
	spec := &kcp.Spec.Template.Spec.KubeadmConfigSpec
	spec.ClusterConfiguration.APIServer.ExtraArgs = []bootstrapv1.Arg{
		{Name: "admission-control-config-file", Value: ptr.To(defaultAdmissionConfigPath)},
		{Name: "enable-admission-plugins", Value: ptr.To("PodSecurity")},
	}
	spec.Files = []bootstrapv1.File{
		{Path: defaultAdmissionConfigPath, Content: existingAdmissionConfig},
	}

	err := AddPlugin(kcp, Plugin{
		Name:              "PodSecurity",
		ConfigFilePath:    "/etc/kubernetes/pod-security-admission.yaml",
		ConfigFileContent: "test-content",
	})
	require.NoError(t, err)

	// Should not duplicate PodSecurity in plugins list
	admissionFile := findFile(spec.Files, defaultAdmissionConfigPath)
	require.NotNil(t, admissionFile)
	// Count occurrences of "PodSecurity" as plugin name
	assert.Equal(t, 1, countPluginOccurrences(admissionFile.Content, "PodSecurity"))
}

func TestAddPlugin_EnableAdmissionPluginsDeduplication(t *testing.T) {
	kcp := &controlplanev1.KubeadmControlPlaneTemplate{}
	spec := &kcp.Spec.Template.Spec.KubeadmConfigSpec
	spec.ClusterConfiguration.APIServer.ExtraArgs = []bootstrapv1.Arg{
		{Name: "enable-admission-plugins", Value: ptr.To("PodSecurity,NodeRestriction")},
	}

	err := AddPlugin(kcp, Plugin{
		Name:              "PodSecurity",
		ConfigFilePath:    "/etc/kubernetes/pod-security-admission.yaml",
		ConfigFileContent: "test-content",
	})
	require.NoError(t, err)

	// Should not duplicate PodSecurity in enable-admission-plugins
	for _, arg := range spec.ClusterConfiguration.APIServer.ExtraArgs {
		if arg.Name == "enable-admission-plugins" {
			assert.Equal(t, "PodSecurity,NodeRestriction", *arg.Value)
		}
	}
}

// Test helpers

func assertFileExists(t *testing.T, files []bootstrapv1.File, path string) {
	t.Helper()
	assert.NotNil(t, findFile(files, path), "file %s not found", path)
}

func findFile(files []bootstrapv1.File, path string) *bootstrapv1.File {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}

func assertVolumeMountExists(t *testing.T, volumes []bootstrapv1.HostPathMount, path string) {
	t.Helper()
	for _, v := range volumes {
		if v.MountPath == path {
			return
		}
	}
	t.Errorf("volume mount for %s not found", path)
}

func assertAdmissionPluginEnabled(t *testing.T, args []bootstrapv1.Arg, plugin string) {
	t.Helper()
	for _, arg := range args {
		if arg.Name == "enable-admission-plugins" && arg.Value != nil {
			assert.Contains(t, *arg.Value, plugin)
			return
		}
	}
	t.Errorf("enable-admission-plugins arg not found or does not contain %s", plugin)
}

func countPluginOccurrences(content, pluginName string) int {
	count := 0
	// Simple count: each "- name: <pluginName>" line is one occurrence
	for _, line := range splitLines(content) {
		if contains(line, "name: "+pluginName) {
			count++
		}
	}
	return count
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd pkg/handlers/generic/mutation/kubeadm/admissionconfiguration && go test ./...`

Expected: Compilation failure — `AddPlugin`, `Plugin`, `defaultAdmissionConfigPath` not defined.

- [ ] **Step 3: Implement the shared package**

Create `pkg/handlers/generic/mutation/kubeadm/admissionconfiguration/admission_configuration.go`:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package admissionconfiguration

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	"sigs.k8s.io/yaml"
)

const defaultAdmissionConfigPath = "/etc/kubernetes/admission.yaml"

// Plugin describes an admission plugin to add to the API server's AdmissionConfiguration.
type Plugin struct {
	// Name is the admission plugin name (e.g. "PodSecurity", "EventRateLimit").
	Name string
	// ConfigFilePath is where the plugin's own config file is written on the node.
	ConfigFilePath string
	// ConfigFileContent is the serialized plugin configuration.
	ConfigFileContent string
}

type admissionConfiguration struct {
	APIVersion string                  `json:"apiVersion"`
	Kind       string                  `json:"kind"`
	Plugins    []admissionConfigPlugin `json:"plugins"`
}

type admissionConfigPlugin struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// AddPlugin adds an admission plugin to the KubeadmControlPlaneTemplate. It handles:
// 1. Creating or updating the AdmissionConfiguration file
// 2. Adding the plugin's own config file
// 3. Adding volume mounts for both files
// 4. Adding the plugin to enable-admission-plugins
func AddPlugin(
	kcp *controlplanev1.KubeadmControlPlaneTemplate,
	plugin Plugin,
) error {
	spec := &kcp.Spec.Template.Spec.KubeadmConfigSpec
	apiServer := &spec.ClusterConfiguration.APIServer

	admissionConfigPath := getAdmissionConfigPath(apiServer.ExtraArgs)

	if err := addOrUpdateAdmissionConfig(spec, admissionConfigPath, plugin); err != nil {
		return fmt.Errorf("failed to update admission configuration: %w", err)
	}

	addPluginConfigFile(spec, plugin)

	addVolumeMountIfMissing(apiServer, admissionConfigPath, "admission-config")
	addVolumeMountIfMissing(apiServer, plugin.ConfigFilePath, sanitizeVolumeName(plugin.Name))

	setAdmissionConfigArg(apiServer, admissionConfigPath)
	addToEnabledPlugins(apiServer, plugin.Name)

	return nil
}

func getAdmissionConfigPath(args []bootstrapv1.Arg) string {
	for _, arg := range args {
		if arg.Name == "admission-control-config-file" && arg.Value != nil {
			return *arg.Value
		}
	}
	return defaultAdmissionConfigPath
}

func addOrUpdateAdmissionConfig(
	spec *bootstrapv1.KubeadmConfigSpec,
	admissionConfigPath string,
	plugin Plugin,
) error {
	for i := range spec.Files {
		if spec.Files[i].Path == admissionConfigPath {
			return appendPluginToExistingConfig(&spec.Files[i], plugin)
		}
	}

	config := admissionConfiguration{
		APIVersion: "apiserver.config.k8s.io/v1",
		Kind:       "AdmissionConfiguration",
		Plugins: []admissionConfigPlugin{
			{Name: plugin.Name, Path: plugin.ConfigFilePath},
		},
	}
	content, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal admission configuration: %w", err)
	}

	spec.Files = append(spec.Files, bootstrapv1.File{
		Path:        admissionConfigPath,
		Permissions: "0600",
		Content:     string(content),
	})

	return nil
}

func appendPluginToExistingConfig(file *bootstrapv1.File, plugin Plugin) error {
	var config admissionConfiguration
	if err := yaml.Unmarshal([]byte(file.Content), &config); err != nil {
		return fmt.Errorf("failed to parse existing admission configuration: %w", err)
	}

	for _, p := range config.Plugins {
		if p.Name == plugin.Name {
			return nil
		}
	}

	config.Plugins = append(config.Plugins, admissionConfigPlugin{
		Name: plugin.Name,
		Path: plugin.ConfigFilePath,
	})

	content, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal updated admission configuration: %w", err)
	}
	file.Content = string(content)

	return nil
}

func addPluginConfigFile(spec *bootstrapv1.KubeadmConfigSpec, plugin Plugin) {
	for _, f := range spec.Files {
		if f.Path == plugin.ConfigFilePath {
			return
		}
	}
	spec.Files = append(spec.Files, bootstrapv1.File{
		Path:        plugin.ConfigFilePath,
		Permissions: "0600",
		Content:     plugin.ConfigFileContent,
	})
}

func addVolumeMountIfMissing(
	apiServer *bootstrapv1.APIServer,
	path string,
	name string,
) {
	for _, v := range apiServer.ExtraVolumes {
		if v.MountPath == path {
			return
		}
	}
	apiServer.ExtraVolumes = append(apiServer.ExtraVolumes, bootstrapv1.HostPathMount{
		Name:      name,
		HostPath:  path,
		MountPath: path,
		ReadOnly:  ptr.To(true),
		PathType:  corev1.HostPathFile,
	})
}

func setAdmissionConfigArg(apiServer *bootstrapv1.APIServer, path string) {
	for _, arg := range apiServer.ExtraArgs {
		if arg.Name == "admission-control-config-file" {
			return
		}
	}
	apiServer.ExtraArgs = append(apiServer.ExtraArgs, bootstrapv1.Arg{
		Name:  "admission-control-config-file",
		Value: ptr.To(path),
	})
}

func addToEnabledPlugins(apiServer *bootstrapv1.APIServer, pluginName string) {
	for i, arg := range apiServer.ExtraArgs {
		if arg.Name == "enable-admission-plugins" && arg.Value != nil {
			plugins := strings.Split(*arg.Value, ",")
			for _, p := range plugins {
				if strings.TrimSpace(p) == pluginName {
					return
				}
			}
			apiServer.ExtraArgs[i].Value = ptr.To(*arg.Value + "," + pluginName)
			return
		}
	}
	apiServer.ExtraArgs = append(apiServer.ExtraArgs, bootstrapv1.Arg{
		Name:  "enable-admission-plugins",
		Value: ptr.To(pluginName),
	})
}

func sanitizeVolumeName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "-config"
}
```

Note: the `text/template` import and `bytes` import should be removed if unused after implementation — they were included speculatively. Review imports after writing.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd pkg/handlers/generic/mutation/kubeadm/admissionconfiguration && go test -v ./...`

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/admissionconfiguration/
git commit -m "feat: [NCN-101538] Add shared admissionconfiguration package

Reusable package for adding admission plugins to the API server's
AdmissionConfiguration. Handles existing config files, extra args,
volume mounts, and enable-admission-plugins deduplication."
```

---

## Task 3: Create the PSA handler

**Files:**
- Create: `pkg/handlers/generic/mutation/kubeadm/podsecurityadmission/inject.go`
- Modify: `pkg/handlers/generic/mutation/handlers.go:34-65`

- [ ] **Step 1: Write the test file first**

Create `pkg/handlers/generic/mutation/kubeadm/podsecurityadmission/inject_test.go`:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package podsecurityadmission

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestPodSecurityAdmissionPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Pod Security Admission mutator suite")
}

var _ = Describe("Generate Pod Security Admission patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler(
			"", helpers.TestEnv.Client, NewPatch(),
		).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name:                  "variable not set results in no patches",
			RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{},
		},
		{
			Name:        "enforce restricted with defaults",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", psaConfigFilePath,
						),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.PodSecurityAdmission{
						Enforce: v1alpha1.PodSecurityStandardRestricted,
						Audit:   v1alpha1.PodSecurityStandardPrivileged,
						Warn:    v1alpha1.PodSecurityStandardPrivileged,
						Exemptions: v1alpha1.PodSecurityExemptions{
							Namespaces: []string{"kube-system"},
						},
					},
					VariableName,
				),
			},
		},
		{
			Name:        "all modes set to restricted",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", psaConfigFilePath,
						),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.PodSecurityAdmission{
						Enforce: v1alpha1.PodSecurityStandardRestricted,
						Audit:   v1alpha1.PodSecurityStandardRestricted,
						Warn:    v1alpha1.PodSecurityStandardRestricted,
						Exemptions: v1alpha1.PodSecurityExemptions{
							Namespaces: []string{"kube-system"},
						},
					},
					VariableName,
				),
			},
		},
		{
			Name:        "custom exemptions",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", psaConfigFilePath,
						),
					),
				},
			},
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.PodSecurityAdmission{
						Enforce: v1alpha1.PodSecurityStandardRestricted,
						Audit:   v1alpha1.PodSecurityStandardRestricted,
						Warn:    v1alpha1.PodSecurityStandardRestricted,
						Exemptions: v1alpha1.PodSecurityExemptions{
							Namespaces:        []string{"kube-system", "my-privileged-ns"},
							Usernames:         []string{"system:serviceaccount:kube-system:some-sa"},
							RuntimeClassNames: []string{"kata"},
						},
					},
					VariableName,
				),
			},
		},
	}

	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd pkg/handlers/generic/mutation/kubeadm/podsecurityadmission && go test ./...`

Expected: Compilation failure — `NewPatch`, `VariableName`, `psaConfigFilePath` not defined.

- [ ] **Step 3: Implement the PSA handler**

Create `pkg/handlers/generic/mutation/kubeadm/podsecurityadmission/inject.go`:

```go
// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package podsecurityadmission

import (
	"bytes"
	"context"
	"text/template"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/admissionconfiguration"
)

const (
	VariableName     = "podSecurityAdmission"
	psaConfigFilePath = "/etc/kubernetes/pod-security-admission.yaml"
	pluginName        = "PodSecurity"
)

var psaConfigTemplate = template.Must(template.New("psa").Parse(`apiVersion: pod-security.admission.config.k8s.io/v1
kind: PodSecurityConfiguration
defaults:
  enforce: "{{ .Enforce }}"
  enforce-version: "latest"
  audit: "{{ .Audit }}"
  audit-version: "latest"
  warn: "{{ .Warn }}"
  warn-version: "latest"
exemptions:
  namespaces:{{ range .Exemptions.Namespaces }}
    - "{{ . }}"{{ end }}
  usernames:{{ range .Exemptions.Usernames }}
    - "{{ . }}"{{ end }}
  runtimeClassNames:{{ range .Exemptions.RuntimeClassNames }}
    - "{{ . }}"{{ end }}
`))

type psaPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *psaPatchHandler {
	return &psaPatchHandler{
		variableName:      v1alpha1.ClusterConfigVariableName,
		variableFieldPath: []string{VariableName},
	}
}

func (h *psaPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	psa, err := variables.Get[*v1alpha1.PodSecurityAdmission](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Pod Security Admission variable not defined")
			return nil
		}
		return err
	}

	if psa == nil {
		log.V(5).Info("Pod Security Admission not specified, skipping mutation")
		return nil
	}

	log = log.WithValues(
		"variableName", h.variableName,
		"variableFieldPath", h.variableFieldPath,
		"variableValue", psa,
	)

	configContent, err := generatePSAConfig(psa)
	if err != nil {
		return err
	}

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding Pod Security Admission configuration")

			return admissionconfiguration.AddPlugin(obj, admissionconfiguration.Plugin{
				Name:              pluginName,
				ConfigFilePath:    psaConfigFilePath,
				ConfigFileContent: configContent,
			})
		},
	)
}

func generatePSAConfig(psa *v1alpha1.PodSecurityAdmission) (string, error) {
	var buf bytes.Buffer
	if err := psaConfigTemplate.Execute(&buf, psa); err != nil {
		return "", err
	}
	return buf.String(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd pkg/handlers/generic/mutation/kubeadm/podsecurityadmission && go test -v ./...`

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add pkg/handlers/generic/mutation/kubeadm/podsecurityadmission/
git commit -m "feat: [NCN-101538] Add PodSecurity admission handler

Thin handler that reads the podSecurityAdmission variable and generates
a PodSecurityConfiguration file, delegating to the shared
admissionconfiguration package for API server integration."
```

---

## Task 4: Register the handler

**Files:**
- Modify: `pkg/handlers/generic/mutation/handlers.go:34-65`

- [ ] **Step 1: Add the import and handler registration**

In `pkg/handlers/generic/mutation/handlers.go`, add the import:

```go
"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/podsecurityadmission"
```

In the `MetaMutators()` function, add `podsecurityadmission.NewPatch()` after `kubeproxymode.NewPatch()` and before `ntp.NewPatch()`:

```go
kubeproxymode.NewPatch(),
podsecurityadmission.NewPatch(),
ntp.NewPatch(),
```

- [ ] **Step 2: Run existing tests to verify no regressions**

Run: `cd pkg/handlers/generic/mutation && go test ./...`

Expected: All existing tests pass. No regressions.

- [ ] **Step 3: Commit**

```bash
git add pkg/handlers/generic/mutation/handlers.go
git commit -m "feat: [NCN-101538] Register PodSecurity admission handler

Add podsecurityadmission handler to MetaMutators in the generic
mutation handler chain."
```

---

## Task 5: Run full code generation and tests

- [ ] **Step 1: Run code generation**

Run: `make go-generate`

Expected: Succeeds. CRD schemas updated with `podSecurityAdmission` field.

- [ ] **Step 2: Run all tests**

Run: `make test`

Expected: All tests pass, including the new PSA handler tests and the shared package tests.

- [ ] **Step 3: Run linting**

Run: `make lint`

Expected: No new lint errors.

- [ ] **Step 4: Commit any generated file changes**

```bash
git add -A
git commit -m "build: [NCN-101538] Regenerate CRDs and deepcopy for PSA types"
```

---

## Task 6: Add documentation

**Files:**
- Create: `docs/content/customization/kubeadm/pod-security-admission.md`

- [ ] **Step 1: Create the documentation page**

Create `docs/content/customization/kubeadm/pod-security-admission.md`:

```markdown
+++
title = "Pod Security Admission"
+++

Configure cluster-wide [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/)
defaults via the `podSecurityAdmission` variable. This configures the `PodSecurity` admission plugin on the
API server with default enforce, audit, and warn levels applied to all namespaces that do not have their own
pod security labels.

This feature is available for kubeadm-based providers (AWS, Docker, Nutanix) only. It is not supported on EKS,
which uses a managed control plane.

When `podSecurityAdmission` is not specified, no PodSecurity admission configuration is applied.
Existing clusters are not affected.

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enforce` | `privileged` \| `baseline` \| `restricted` | `privileged` | Pods violating this level are rejected. |
| `audit` | `privileged` \| `baseline` \| `restricted` | `privileged` | Violations are recorded in the API server audit log. |
| `warn` | `privileged` \| `baseline` \| `restricted` | `privileged` | Violations trigger a user-facing warning. |
| `exemptions.namespaces` | `[]string` | `["kube-system"]` | Namespaces exempt from enforcement. |
| `exemptions.usernames` | `[]string` | `[]` | Usernames exempt from enforcement. |
| `exemptions.runtimeClassNames` | `[]string` | `[]` | RuntimeClassNames exempt from enforcement. |

The version for all modes is always set to `latest`.

## Example

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          podSecurityAdmission:
            enforce: restricted
            audit: restricted
            warn: restricted
            exemptions:
              namespaces:
                - kube-system
                - my-privileged-ns
              usernames:
                - system:serviceaccount:kube-system:some-sa
```

Applying this configuration will result in the following changes to the `KubeadmControlPlaneTemplate`:

1. A `PodSecurityConfiguration` file at `/etc/kubernetes/pod-security-admission.yaml`:

```yaml
apiVersion: pod-security.admission.config.k8s.io/v1
kind: PodSecurityConfiguration
defaults:
  enforce: "restricted"
  enforce-version: "latest"
  audit: "restricted"
  audit-version: "latest"
  warn: "restricted"
  warn-version: "latest"
exemptions:
  namespaces:
    - "kube-system"
    - "my-privileged-ns"
  usernames:
    - "system:serviceaccount:kube-system:some-sa"
  runtimeClassNames: []
```

2. An `AdmissionConfiguration` file (or updated existing one) referencing the PodSecurity plugin.

3. The `PodSecurity` plugin added to `--enable-admission-plugins`.

4. The `--admission-control-config-file` API server argument set (if not already present).
```

- [ ] **Step 2: Commit**

```bash
git add docs/content/customization/kubeadm/pod-security-admission.md
git commit -m "docs: [NCN-101538] Add Pod Security Admission documentation"
```
