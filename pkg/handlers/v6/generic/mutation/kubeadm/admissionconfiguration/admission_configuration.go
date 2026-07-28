// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package admissionconfiguration

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	"sigs.k8s.io/yaml"
)

const DefaultAdmissionConfigPath = "/etc/kubernetes/admission.yaml"

// Plugin describes an admission plugin to add to the API server's AdmissionConfiguration.
type Plugin struct {
	Name              string
	ConfigFilePath    string
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
//   - Creating or updating the AdmissionConfiguration file
//   - Adding the plugin's own config file
//   - Adding volume mounts for both files
//   - Adding the plugin to enable-admission-plugins
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
	return DefaultAdmissionConfigPath
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
			for p := range strings.SplitSeq(*arg.Value, ",") {
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
