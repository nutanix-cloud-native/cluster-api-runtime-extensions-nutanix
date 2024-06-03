// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/blang/semver/v4"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common"
)

const (
	kubeVIPFileOwner       = "root:root"
	kubeVIPFilePath        = "/etc/kubernetes/manifests/kube-vip.yaml"
	kubeVIPFilePermissions = "0600"

	configureKubeVIPScriptPermissions = "0700"
)

var (
	configureKubeVIPScriptOnRemote = common.ConfigFilePathOnRemote(
		"configure-kube-vip.sh")

	configureKubeVIPScriptOnRemotePreKubeadmCommand  = "/bin/bash " + configureKubeVIPScriptOnRemote + " use-super-admin.conf"
	configureKubeVIPScriptOnRemotePostKubeadmCommand = "/bin/bash " + configureKubeVIPScriptOnRemote + " use-admin.conf"
)

var (
	//go:embed templates/configure-kube-vip.sh
	configureKubeVIPScript []byte
)

type kubeVIPFromConfigMapProvider struct {
	client client.Reader

	configMapKey client.ObjectKey
}

func NewKubeVIPFromConfigMapProvider(
	cl client.Reader,
	name, namespace string,
) *kubeVIPFromConfigMapProvider {
	return &kubeVIPFromConfigMapProvider{
		client: cl,
		configMapKey: client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (p *kubeVIPFromConfigMapProvider) Name() string {
	return "kube-vip"
}

// GenerateFilesAndCommands returns files and pre/post kubeadm commands for kube-vip.
// It reads kube-vip template from a ConfigMap and returns the content a File, templating the required variables.
// If required, it also returns a script file and pre/post kubeadm commands to change the kube-vip Pod to use the new
// super-admin.conf file.
func (p *kubeVIPFromConfigMapProvider) GenerateFilesAndCommands(
	ctx context.Context,
	spec v1alpha1.ControlPlaneEndpointSpec,
	cluster *clusterv1.Cluster,
) ([]bootstrapv1.File, []string, []string, error) {
	data, err := getTemplateFromConfigMap(ctx, p.client, p.configMapKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed getting template data: %w", err)
	}

	kubeVIPStaticPod, err := templateValues(spec, data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed templating static Pod: %w", err)
	}

	files := []bootstrapv1.File{
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
	needCommands, err := needHackCommands(cluster)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to determine if kube-vip commands are needed: %w", err)
	}
	if !needCommands {
		return files, nil, nil, nil
	}

	files = append(
		files,
		bootstrapv1.File{
			Content:     string(configureKubeVIPScript),
			Path:        configureKubeVIPScriptOnRemote,
			Permissions: configureKubeVIPScriptPermissions,
		},
	)
	preKubeadmCommands := []string{configureKubeVIPScriptOnRemotePreKubeadmCommand}
	postKubeadmCommands := []string{configureKubeVIPScriptOnRemotePostKubeadmCommand}

	return files, preKubeadmCommands, postKubeadmCommands, nil
}

type multipleKeysError struct {
	configMapKey client.ObjectKey
}

func (e multipleKeysError) Error() string {
	return fmt.Sprintf(
		"found multiple keys in ConfigMap %q, when only 1 is expected",
		e.configMapKey,
	)
}

type emptyValuesError struct {
	configMapKey client.ObjectKey
}

func (e emptyValuesError) Error() string {
	return fmt.Sprintf(
		"could not find any keys with non-empty vaules in ConfigMap %q",
		e.configMapKey,
	)
}

func getTemplateFromConfigMap(
	ctx context.Context,
	cl client.Reader,
	configMapKey client.ObjectKey,
) (string, error) {
	configMap := &corev1.ConfigMap{}
	err := cl.Get(ctx, configMapKey, configMap)
	if err != nil {
		return "", fmt.Errorf("failed to get template ConfigMap %q: %w", configMapKey, err)
	}

	if len(configMap.Data) > 1 {
		return "", multipleKeysError{configMapKey: configMapKey}
	}

	// at this point there should only be 1 key ConfigMap, return on the first non-empty value
	for _, data := range configMap.Data {
		if data != "" {
			return data, nil
		}
	}

	return "", emptyValuesError{configMapKey: configMapKey}
}

func needHackCommands(cluster *clusterv1.Cluster) (bool, error) {
	version, err := semver.ParseTolerant(cluster.Spec.Topology.Version)
	if err != nil {
		return false, fmt.Errorf("failed to parse version from cluster: %w", err)
	}

	return version.Minor >= 29, nil
}
