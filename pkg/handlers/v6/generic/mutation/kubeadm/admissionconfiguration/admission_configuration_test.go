// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package admissionconfiguration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	assertArgValue(t, spec.ClusterConfiguration.APIServer.ExtraArgs,
		"admission-control-config-file", DefaultAdmissionConfigPath)
	assertFileExists(t, spec.Files, DefaultAdmissionConfigPath)
	assertFileExists(t, spec.Files, "/etc/kubernetes/pod-security-admission.yaml")
	assertVolumeMountExists(t, spec.ClusterConfiguration.APIServer.ExtraVolumes, DefaultAdmissionConfigPath)
	assertVolumeMountExists(t, spec.ClusterConfiguration.APIServer.ExtraVolumes,
		"/etc/kubernetes/pod-security-admission.yaml")
	assertAdmissionPluginEnabled(t, spec.ClusterConfiguration.APIServer.ExtraArgs, "PodSecurity")
}

func TestAddPlugin_ExistingAdmissionConfigFileAndArg(t *testing.T) {
	customPath := "/custom/admission/config.yaml"
	existingAdmissionConfig := `apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: EventRateLimit
  path: /etc/kubernetes/eventratelimit-config.yaml
`
	kcp := &controlplanev1.KubeadmControlPlaneTemplate{}
	spec := &kcp.Spec.Template.Spec.KubeadmConfigSpec
	spec.ClusterConfiguration.APIServer.ExtraArgs = []bootstrapv1.Arg{
		{Name: "admission-control-config-file", Value: ptr.To(customPath)},
		{Name: "enable-admission-plugins", Value: ptr.To("EventRateLimit,NodeRestriction")},
	}
	spec.Files = []bootstrapv1.File{
		{Path: customPath, Content: existingAdmissionConfig},
	}

	err := AddPlugin(kcp, Plugin{
		Name:              "PodSecurity",
		ConfigFilePath:    "/etc/kubernetes/pod-security-admission.yaml",
		ConfigFileContent: "test-content",
	})
	require.NoError(t, err)

	assertArgValue(t, spec.ClusterConfiguration.APIServer.ExtraArgs,
		"admission-control-config-file", customPath)

	admissionFile := findFile(spec.Files, customPath)
	require.NotNil(t, admissionFile)
	assert.Contains(t, admissionFile.Content, "EventRateLimit")
	assert.Contains(t, admissionFile.Content, "PodSecurity")

	assertFileExists(t, spec.Files, "/etc/kubernetes/pod-security-admission.yaml")
	assertVolumeMountExists(t, spec.ClusterConfiguration.APIServer.ExtraVolumes, customPath)
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

	assertFileExists(t, spec.Files, customPath)
	assertArgValue(t, spec.ClusterConfiguration.APIServer.ExtraArgs,
		"admission-control-config-file", customPath)
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
		{Name: "admission-control-config-file", Value: ptr.To(DefaultAdmissionConfigPath)},
		{Name: "enable-admission-plugins", Value: ptr.To("PodSecurity")},
	}
	spec.Files = []bootstrapv1.File{
		{Path: DefaultAdmissionConfigPath, Content: existingAdmissionConfig},
	}

	err := AddPlugin(kcp, Plugin{
		Name:              "PodSecurity",
		ConfigFilePath:    "/etc/kubernetes/pod-security-admission.yaml",
		ConfigFileContent: "test-content",
	})
	require.NoError(t, err)

	admissionFile := findFile(spec.Files, DefaultAdmissionConfigPath)
	require.NotNil(t, admissionFile)
	assert.Equal(t, 1, strings.Count(admissionFile.Content, "name: PodSecurity"))
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

	assertArgValue(t, spec.ClusterConfiguration.APIServer.ExtraArgs,
		"enable-admission-plugins", "PodSecurity,NodeRestriction")
}

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

func assertArgValue(t *testing.T, args []bootstrapv1.Arg, name, expectedValue string) {
	t.Helper()
	for _, arg := range args {
		if arg.Name == name {
			require.NotNil(t, arg.Value, "arg %s has nil value", name)
			assert.Equal(t, expectedValue, *arg.Value)
			return
		}
	}
	t.Errorf("arg %s not found", name)
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
