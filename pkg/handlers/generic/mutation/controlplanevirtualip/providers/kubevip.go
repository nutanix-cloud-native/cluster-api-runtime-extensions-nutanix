// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/blang/semver/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

const (
	kubeVIPFileOwner       = "root:root"
	kubeVIPFilePath        = "/etc/kubernetes/manifests/kube-vip.yaml"
	kubeVIPFilePermissions = "0600"

	configureForKubeVIPScriptPermissions = "0700"
)

var (
	configureForKubeVIPScriptOnRemote = common.ConfigFilePathOnRemote(
		"configure-for-kube-vip.sh")

	configureForKubeVIPScriptOnRemotePreKubeadmCommand = "/bin/bash " +
		configureForKubeVIPScriptOnRemote + " set-host-aliases use-super-admin.conf"
	configureForKubeVIPScriptOnRemotePostKubeadmCommand = "/bin/bash " +
		configureForKubeVIPScriptOnRemote + " use-admin.conf"
)

//go:embed templates/configure-for-kube-vip.sh
var configureForKubeVIPScript []byte

type kubeVIPFromKCPTemplateProvider struct {
	template *controlplanev1.KubeadmControlPlaneTemplate
}

func NewKubeVIPFromKCPTemplateProvider(
	template *controlplanev1.KubeadmControlPlaneTemplate,
) *kubeVIPFromKCPTemplateProvider {
	return &kubeVIPFromKCPTemplateProvider{
		template: template,
	}
}

func (p *kubeVIPFromKCPTemplateProvider) Name() string {
	return "kube-vip"
}

// GenerateFilesAndCommands returns files and pre/post kubeadm commands for kube-vip.
// It reads kube-vip template from a ConfigMap and returns the content a File, templating the required variables.
// If required, it also returns a script file and pre/post kubeadm commands to change the kube-vip Pod to use the new
// super-admin.conf file.
func (p *kubeVIPFromKCPTemplateProvider) GenerateFilesAndCommands(
	_ context.Context,
	spec v1alpha1.ControlPlaneEndpointSpec,
	cluster *clusterv1.Cluster,
) (files []bootstrapv1.File, preKubeadmCommands, postKubeadmCommands []string, err error) {
	data, err := getTemplate(p.template)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed getting template data: %w", err)
	}

	kubeVIPStaticPod, err := templateValues(spec, data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed templating static Pod: %w", err)
	}

	files = []bootstrapv1.File{
		{
			Content:     kubeVIPStaticPod,
			Owner:       kubeVIPFileOwner,
			Path:        kubeVIPFilePath,
			Permissions: kubeVIPFilePermissions,
		},
	}

	// The kube-vip static Pod uses admin.conf on the host to connect to the API server.
	// But, starting with Kubernetes 1.29, admin.conf first gets created with no RBAC permissions.
	// At the same time, 'kubeadm init' command waits for the API server to be reachable on the kube-vip IP.
	// And since the kube-vip Pod is crashlooping with a permissions error, 'kubeadm init' fails.
	// To work around this:
	// 1. return a preKubeadmCommand to change the kube-vip Pod to use the new super-admin.conf file.
	// 2. return a postKubeadmCommand to change the kube-vip Pod back to use admin.conf,
	// after kubeadm has assigned it the necessary RBAC permissions.
	//
	// See https://github.com/kube-vip/kube-vip/issues/684
	//
	// There is also another issue introduced in Kubernetes 1.29.
	// If a cloud provider did not yet initialise the node's .status.addresses,
	// the code for creating the /etc/hosts file including the hostAliases does not get run.
	// The kube-vip static Pod runs before the cloud provider and will not be able to resolve the kubernetes DNS name.
	// To work around this:
	// 1. return a preKubeadmCommand to add kubernetes DNS name to /etc/hosts.
	//
	// See https://github.com/kube-vip/kube-vip/issues/692
	// See https://github.com/kubernetes/kubernetes/issues/122420#issuecomment-1864609518
	needCommands, err := needHackCommands(cluster)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"failed to determine if kube-vip commands are needed: %w",
			err,
		)
	}
	if !needCommands {
		return files, nil, nil, nil
	}

	files = append(
		files,
		bootstrapv1.File{
			Content:     string(configureForKubeVIPScript),
			Path:        configureForKubeVIPScriptOnRemote,
			Permissions: configureForKubeVIPScriptPermissions,
		},
	)

	preKubeadmCommands = []string{configureForKubeVIPScriptOnRemotePreKubeadmCommand}
	postKubeadmCommands = []string{configureForKubeVIPScriptOnRemotePostKubeadmCommand}

	return files, preKubeadmCommands, postKubeadmCommands, nil
}

type missingTemplateError struct {
	path string
}

func (e missingTemplateError) Error() string {
	return fmt.Sprintf(
		"did not find kube-vip template file %q in KubeadmControlPlaneTemplate",
		e.path,
	)
}

func getTemplate(kcp *controlplanev1.KubeadmControlPlaneTemplate) (string, error) {
	for _, file := range kcp.Spec.Template.Spec.KubeadmConfigSpec.Files {
		if file.Path == kubeVIPFilePath && file.Content != "" {
			return file.Content, nil
		}
	}

	return "", missingTemplateError{path: kubeVIPFilePath}
}

func needHackCommands(cluster *clusterv1.Cluster) (bool, error) {
	version, err := semver.ParseTolerant(cluster.Spec.Topology.Version)
	if err != nil {
		return false, fmt.Errorf("failed to parse version from cluster: %w", err)
	}

	return version.Minor >= 29, nil
}
