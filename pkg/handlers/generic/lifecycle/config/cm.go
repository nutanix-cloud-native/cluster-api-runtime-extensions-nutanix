package config

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type HelmConfig struct {
	cl          ctrlclient.Reader
	cmName      string
	cmNamespace string
}

type HelmSettings struct {
	HelmChartName       string `yaml:"defaultChartName"`
	HelmChartVersion    string `yaml:"defaultChartVersion"`
	HelmChartRepository string `yaml:"defaultRepositoryUrl"`
}

func NewHelmConfigFromConfigMap(cmName, cmNamespace string, cl ctrlclient.Reader) *HelmConfig {
	return &HelmConfig{
		cl,
		cmName,
		cmNamespace,
	}
}

func (h *HelmConfig) get(
	ctx context.Context,
) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.cmNamespace,
			Name:      h.cmName,
		},
	}
	err := h.cl.Get(
		ctx,
		ctrlclient.ObjectKeyFromObject(cm),
		cm,
	)
	return cm, err
}

func (h *HelmConfig) GetSettingsFor(
	ctx context.Context,
	componentSettings string,
) (*HelmSettings, error) {
	cm, err := h.get(ctx)
	if err != nil {
		return nil, err
	}
	d, ok := cm.Data[componentSettings]
	if !ok {
		return nil, fmt.Errorf("did not find key %s in %v", componentSettings, cm.Data)
	}
	var settings HelmSettings
	err = yaml.Unmarshal([]byte(d), &settings)
	return &settings, err
}
