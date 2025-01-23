// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

type Component string

const (
	Autoscaler              Component = "cluster-autoscaler"
	Tigera                  Component = "tigera-operator"
	Cilium                  Component = "cilium"
	NFD                     Component = "nfd"
	NutanixStorageCSI       Component = "nutanix-storage-csi"
	SnapshotController      Component = "snapshot-controller"
	NutanixCCM              Component = "nutanix-ccm"
	MetalLB                 Component = "metallb"
	LocalPathProvisionerCSI Component = "local-path-provisioner-csi"
	AWSEBSCSI               Component = "aws-ebs-csi"
	AWSCCM                  Component = "aws-ccm"
	COSIController          Component = "cosi-controller"
)

type HelmChartGetter struct {
	cl          ctrlclient.Reader
	cmName      string
	cmNamespace string
}

type HelmChart struct {
	Name       string `yaml:"ChartName"`
	Version    string `yaml:"ChartVersion"`
	Repository string `yaml:"RepositoryURL"`
}

func NewHelmChartGetterFromConfigMap(
	cmName, cmNamespace string,
	cl ctrlclient.Reader,
) *HelmChartGetter {
	return &HelmChartGetter{
		cl,
		cmName,
		cmNamespace,
	}
}

func (h *HelmChartGetter) get(
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

type HelmChartNotFoundError struct {
	Component Component
	Name      string
	Namespace string
}

func (e HelmChartNotFoundError) Error() string {
	return fmt.Sprintf(
		"did not find Helm Chart component %q in configmap %s/%s",
		e.Component,
		e.Namespace,
		e.Name,
	)
}

func (h *HelmChartGetter) getInfoFor(
	ctx context.Context,
	log logr.Logger,
	name Component,
) (*HelmChart, error) {
	cm, err := h.get(ctx)
	if err != nil {
		return nil, err
	}
	d, ok := cm.Data[string(name)]
	if !ok {
		return nil, HelmChartNotFoundError{
			name,
			h.cmNamespace,
			h.cmName,
		}
	}
	var settings HelmChart
	err = yaml.Unmarshal([]byte(d), &settings)
	if err != nil {
		log.Info(
			fmt.Sprintf("Using HelmChart info for %q from configmap %s/%s",
				string(name),
				h.cmNamespace,
				h.cmName),
		)
	}
	return &settings, err
}

func (h *HelmChartGetter) For(
	ctx context.Context,
	log logr.Logger,
	cluster *clusterv1.Cluster,
	name Component,
) (*HelmChart, error) {
	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)

	helmChartConfig, err := variables.Get[v1alpha1.HelmChartConfig](
		varMap,
		v1alpha1.ClusterConfigVariableName,
		"addons", "helmChartConfig")
	if err != nil {
		if variables.IsNotFoundError(err) {
			// HelmChartConfig is not defined in the cluster. Get the HelmChart from the default configmap.
			return h.getInfoFor(ctx, log, name)
		}
		return nil, err
	}
	// helmChartConfig.ConfigMapRef or helmChartConfig.ConfigMapRef.Name will not be nil
	// because it will be validated by the schema validation.
	cmNameFromVariables := helmChartConfig.ConfigMapRef.Name
	clusterNamespace := cluster.Namespace
	hcGetter := NewHelmChartGetterFromConfigMap(cmNameFromVariables, clusterNamespace, h.cl)
	overrideHelmChart, err := hcGetter.getInfoFor(ctx, log, name)
	if err != nil {
		if errors.As(err, &HelmChartNotFoundError{}) {
			// HelmChart is not defined in the custom configmap. Get the HelmChart from the default configmap.
			return h.getInfoFor(ctx, log, name)
		}
		return nil, err
	}
	return overrideHelmChart, nil
}
