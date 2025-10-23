// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Component string

const (
	Autoscaler                Component = "cluster-autoscaler"
	Tigera                    Component = "tigera-operator"
	Cilium                    Component = "cilium"
	NFD                       Component = "nfd"
	NutanixStorageCSI         Component = "nutanix-storage-csi"
	SnapshotController        Component = "snapshot-controller"
	NutanixCCM                Component = "nutanix-ccm"
	MetalLB                   Component = "metallb"
	LocalPathProvisionerCSI   Component = "local-path-provisioner-csi"
	AWSEBSCSI                 Component = "aws-ebs-csi"
	AWSCCM                    Component = "aws-ccm"
	AWSLoadBalancerController Component = "aws-load-balancer-controller"
	COSIController            Component = "cosi-controller"
	CNCFDistributionRegistry  Component = "cncf-distribution-registry"
	RegistrySyncer            Component = "registry-syncer"
	KonnectorAgent            Component = "konnector-agent"
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

func (h *HelmChartGetter) For(
	ctx context.Context,
	log logr.Logger,
	name Component,
) (*HelmChart, error) {
	log.Info(
		fmt.Sprintf("Fetching HelmChart info for %q from configmap %s/%s",
			string(name),
			h.cmNamespace,
			h.cmName),
	)
	cm, err := h.get(ctx)
	if err != nil {
		return nil, err
	}
	d, ok := cm.Data[string(name)]
	if !ok {
		return nil, fmt.Errorf(
			"did not find key %q in configmap %s/%s",
			name,
			h.cmNamespace,
			h.cmName,
		)
	}
	var settings HelmChart
	err = yaml.Unmarshal([]byte(d), &settings)
	return &settings, err
}
