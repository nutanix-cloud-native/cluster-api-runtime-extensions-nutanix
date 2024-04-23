// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package virtualip

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type KubeVIPFromConfigMapConfig struct {
	*options.GlobalOptions

	defaultConfigMapName string
}

func (c *KubeVIPFromConfigMapConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultConfigMapName,
		prefix+".default-kube-vip-template-configmap-name",
		"default-kube-vip-template",
		"default ConfigMap name that holds the kube-vip template used for the control-plane virtual IP",
	)
}

type kubeVIPFromConfigMapProvider struct {
	client client.Reader

	configMapKey client.ObjectKey
}

func NewKubeVIPFromConfigMapProvider(
	cl client.Reader,
	config *KubeVIPFromConfigMapConfig,
) *kubeVIPFromConfigMapProvider {
	return &kubeVIPFromConfigMapProvider{
		client: cl,
		configMapKey: client.ObjectKey{
			Name:      config.defaultConfigMapName,
			Namespace: config.DefaultsNamespace(),
		},
	}
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
