// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	kubeProxyIsDisabled, err := apivariables.KubeProxyIsDisabled(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to check if kube-proxy is disabled: %w", err)
	}

	funcMap := template.FuncMap{
		"trimPrefix": strings.TrimPrefix,
	}
	ciliumTemplate, err := template.New("").Funcs(funcMap).Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		Provider                   string
		ControlPlaneEndpoint       clusterv1.APIEndpoint
		EnableKubeProxyReplacement bool
	}

	// Assume when kube-proxy is disabled, we should enable Cilium's kube-proxy replacement feature.
	templateInput := input{
		EnableKubeProxyReplacement: kubeProxyIsDisabled,
		Provider:                   capiutils.GetProvider(cluster),
		ControlPlaneEndpoint:       cluster.Spec.ControlPlaneEndpoint,
	}

	var b bytes.Buffer
	err = ciliumTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf(
			"failed templating Cilium values: %w",
			err,
		)
	}

	return b.String(), nil
}

// https://docs.cilium.io/en/stable/operations/upgrade/#running-pre-flight-check-required
func preflightTemplateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	kubeProxyIsDisabled, err := apivariables.KubeProxyIsDisabled(cluster)
	if err != nil {
		return "", fmt.Errorf("failed to check if kube-proxy is disabled: %w", err)
	}

	funcMap := template.FuncMap{
		"trimPrefix": strings.TrimPrefix,
	}
	ciliumPreflightTemplate, err := template.New("").Funcs(funcMap).Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse cilium preflight template: %w", err)
	}

	type input struct {
		ControlPlaneEndpoint       clusterv1.APIEndpoint
		EnableKubeProxyReplacement bool
	}

	// Assume when kube-proxy is disabled, we should enable Cilium's kube-proxy replacement feature.
	templateInput := input{
		ControlPlaneEndpoint:       cluster.Spec.ControlPlaneEndpoint,
		EnableKubeProxyReplacement: kubeProxyIsDisabled,
	}

	var b bytes.Buffer
	err = ciliumPreflightTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf(
			"failed templating Cilium preflight values: %w",
			err,
		)
	}

	return b.String(), nil
}
