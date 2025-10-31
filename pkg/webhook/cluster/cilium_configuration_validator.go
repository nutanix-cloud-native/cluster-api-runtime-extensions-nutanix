// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type advancedCiliumConfigurationValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewAdvancedCiliumConfigurationValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *advancedCiliumConfigurationValidator {
	return &advancedCiliumConfigurationValidator{
		client:  client,
		decoder: decoder,
	}
}

func (a *advancedCiliumConfigurationValidator) Validator() admission.HandlerFunc {
	return a.validate
}

func (a *advancedCiliumConfigurationValidator) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	err := a.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil {
		return admission.Allowed("")
	}

	// Skip validation if the skip annotation is present
	if hasSkipAnnotation(cluster) {
		return admission.Allowed("")
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w",
				v1alpha1.ClusterConfigVariableName,
				err).Error(),
		)
	}

	if clusterConfig == nil {
		return admission.Allowed("")
	}
	// Skip validation if kube-proxy is not disabled.
	if clusterConfig.KubeProxy == nil || clusterConfig.KubeProxy.Mode != v1alpha1.KubeProxyModeDisabled {
		return admission.Allowed("")
	}
	// Skip validation if not using Cilium as CNI provider.
	if clusterConfig.Addons == nil || clusterConfig.Addons.CNI == nil ||
		clusterConfig.Addons.CNI.Provider != v1alpha1.CNIProviderCilium {
		return admission.Allowed("")
	}
	// Skip validation if no custom values are specified.
	if clusterConfig.Addons.CNI.Values == nil || clusterConfig.Addons.CNI.Values.SourceRef == nil {
		return admission.Allowed("")
	}

	// Get the Cilium values from the ConfigMap.
	ciliumValues, err := getCiliumValues(ctx, a.client, cluster, clusterConfig.Addons.CNI)
	if err != nil {
		return admission.Denied(err.Error())
	}
	// Skip validation if no values found.
	if ciliumValues == nil {
		return admission.Allowed("")
	}

	// Validate that kubeProxyReplacement is enabled
	if err := validateCiliumKubeProxyReplacement(ciliumValues, cluster.Namespace, clusterConfig.Addons.CNI.Values.SourceRef.Name); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

func hasSkipAnnotation(cluster *clusterv1.Cluster) bool {
	if cluster.Annotations == nil {
		return false
	}
	val, ok := cluster.Annotations[v1alpha1.SkipCiliumKubeProxyReplacementValidation]
	return ok && val == "true"
}

type ciliumValues struct {
	KubeProxyReplacement bool `json:"kubeProxyReplacement"`
}

// getCiliumValues retrieves and parses the Cilium values from a ConfigMap.
// Returns nil if ConfigMap doesn't exist or values.yaml key is missing.
// Returns error only for actual failures (permission errors, invalid YAML, etc.).
func getCiliumValues(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	cni *v1alpha1.CNI,
) (*ciliumValues, error) {
	configMapName := cni.Values.SourceRef.Name
	configMapNamespace := cluster.Namespace

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: configMapNamespace,
			Name:      configMapName,
		},
	}

	err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(configMap), configMap)
	if err != nil {
		// ConfigMap doesn't exist - this is OK, return nil
		if err = ctrlclient.IgnoreNotFound(err); err != nil {
			return nil, fmt.Errorf("failed to get ConfigMap %s/%s: %w", configMapNamespace, configMapName, err)
		}
		return nil, nil
	}

	// Look for values.yaml key in the ConfigMap
	valuesYAML, ok := configMap.Data["values.yaml"]
	if !ok {
		// values.yaml key doesn't exist - this is OK, return nil
		return nil, nil
	}

	// Unmarshal the YAML
	values := &ciliumValues{}
	if err := yaml.Unmarshal([]byte(valuesYAML), values); err != nil {
		return nil, fmt.Errorf(
			"failed to unmarshal Cilium values from ConfigMap %s/%s: %w",
			configMapNamespace,
			configMapName,
			err,
		)
	}

	return values, nil
}

func validateCiliumKubeProxyReplacement(values *ciliumValues, namespace, configMapName string) error {
	if !values.KubeProxyReplacement {
		return fmt.Errorf(
			"kube-proxy is disabled, but Cilium ConfigMap %s/%s does not have 'kubeProxyReplacement' enabled",
			namespace,
			configMapName,
		)
	}

	return nil
}
