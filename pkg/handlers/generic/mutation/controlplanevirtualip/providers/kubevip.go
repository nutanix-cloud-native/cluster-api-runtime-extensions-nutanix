// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

var (
	//nolint:lll // for readability prefer to keep the long line
	KubeVipPreKubeadmCommands = []string{`if [ -f /run/kubeadm/kubeadm.yaml ]; then
  sed -i 's#path: /etc/kubernetes/admin.conf#path: /etc/kubernetes/super-admin.conf#' /etc/kubernetes/manifests/kube-vip.yaml;
fi`}
	//nolint:lll // for readability prefer to keep the long line
	KubeVipPostKubeadmCommands = []string{`if [ -f /run/kubeadm/kubeadm.yaml ]; then
  sed -i 's#path: /etc/kubernetes/super-admin.conf#path: /etc/kubernetes/admin.conf#' /etc/kubernetes/manifests/kube-vip.yaml;
fi`}
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

// GetFile reads the kube-vip template from the ConfigMap
// and returns the content a File, templating the required variables.
func (p *kubeVIPFromConfigMapProvider) GetFile(
	ctx context.Context,
	spec v1alpha1.ControlPlaneEndpointSpec,
) (*bootstrapv1.File, error) {
	data, err := getTemplateFromConfigMap(ctx, p.client, p.configMapKey)
	if err != nil {
		return nil, fmt.Errorf("failed getting template data: %w", err)
	}

	kubeVIPStaticPod, err := templateValues(spec, data)
	if err != nil {
		return nil, fmt.Errorf("failed templating static Pod: %w", err)
	}

	return &bootstrapv1.File{
		Content:     kubeVIPStaticPod,
		Owner:       kubeVIPFileOwner,
		Path:        kubeVIPFilePath,
		Permissions: kubeVIPFilePermissions,
	}, nil
}

//nolint:gocritic // No need for named return values
func (p *kubeVIPFromConfigMapProvider) GetCommands(cluster *clusterv1.Cluster) ([]string, []string, error) {
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
		return nil, nil, fmt.Errorf("failed to determine if kube-vip commands are needed: %w", err)
	}
	if !needCommands {
		return nil, nil, nil
	}

	return KubeVipPreKubeadmCommands, KubeVipPostKubeadmCommands, nil
}

type multipleKeysError struct {
	configMapKey client.ObjectKey
}

func (e multipleKeysError) Error() string {
	return fmt.Sprintf("found multiple keys in ConfigMap %q, when only 1 is expected", e.configMapKey)
}

type emptyValuesError struct {
	configMapKey client.ObjectKey
}

func (e emptyValuesError) Error() string {
	return fmt.Sprintf("could not find any keys with non-empty vaules in ConfigMap %q", e.configMapKey)
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
		return false, fmt.Errorf("failed to parse version from cluster %w", err)
	}

	return version.Minor >= 29, nil
}
