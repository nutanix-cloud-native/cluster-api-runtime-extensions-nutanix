// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

// evictionThresholdPattern validates eviction threshold values: number with optional decimal,
// optional percentage or binary unit suffix (Ki, Mi, Gi, Ti).
var evictionThresholdPattern = regexp.MustCompile(`^\d+(\.\d+)?(%|Ki|Mi|Gi|Ti)?$`)

type kubeletConfigurationValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewKubeletConfigurationValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *kubeletConfigurationValidator {
	return &kubeletConfigurationValidator{
		client:  client,
		decoder: decoder,
	}
}

func (k *kubeletConfigurationValidator) Validator() admission.HandlerFunc {
	return k.validate
}

func (k *kubeletConfigurationValidator) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	if err := k.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil {
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

	cfg := clusterConfig.KubeletConfiguration
	if cfg == nil {
		return admission.Allowed("")
	}

	// 1. cpuManagerPolicy=static requires CPU reservation
	if cfg.CPUManagerPolicy != nil && *cfg.CPUManagerPolicy == v1alpha1.CPUManagerPolicyStatic {
		hasCPUReserved := hasCPUReservation(cfg.SystemReserved) || hasCPUReservation(cfg.KubeReserved)
		if !hasCPUReserved {
			return admission.Denied(
				"cpuManagerPolicy 'static' requires CPU reservation in systemReserved or kubeReserved",
			)
		}
	}

	// 2. evictionHard/evictionSoft value format validation
	if err := validateEvictionThresholds(cfg.EvictionHard); err != nil {
		return admission.Denied(err.Error())
	}
	if err := validateEvictionThresholds(cfg.EvictionSoft); err != nil {
		return admission.Denied(err.Error())
	}

	var warnings []string
	//nolint:staticcheck // Intentional access to deprecated field for backwards compatibility.
	if clusterConfig.MaxParallelImagePullsPerNode != nil && cfg.MaxParallelImagePulls != nil {
		warnings = append(
			warnings,
			"both maxParallelImagePullsPerNode and "+
				"kubeletConfiguration.maxParallelImagePulls are set; "+
				"maxParallelImagePullsPerNode will be ignored",
		)
	}

	if len(warnings) > 0 {
		return admission.Allowed("").WithWarnings(warnings...)
	}
	return admission.Allowed("")
}

func hasCPUReservation(reserved map[string]resource.Quantity) bool {
	if reserved == nil {
		return false
	}
	_, ok := reserved["cpu"]
	return ok
}

func validateEvictionThresholds(thresholds map[string]string) error {
	if thresholds == nil {
		return nil
	}
	for signal, val := range thresholds {
		if !evictionThresholdPattern.MatchString(val) {
			return fmt.Errorf(
				"invalid eviction threshold value %q for signal %q: must be a percentage or resource quantity",
				val, signal,
			)
		}
	}
	return nil
}
