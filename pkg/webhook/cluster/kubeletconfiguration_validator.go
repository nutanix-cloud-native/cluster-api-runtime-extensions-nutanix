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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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

	if !cluster.Spec.Topology.IsDefined() {
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

	var warnings []string

	cfgsToValidate := []struct {
		cfg  *v1alpha1.KubeletConfiguration
		path string
	}{}

	if clusterConfig.ControlPlane != nil {
		cfgsToValidate = append(cfgsToValidate, struct {
			cfg  *v1alpha1.KubeletConfiguration
			path string
		}{
			clusterConfig.ControlPlane.KubeletConfiguration,
			"clusterConfig.controlPlane.kubeletConfiguration",
		})
	}

	workerConfig, err := variables.UnmarshalWorkerConfigVariable(
		cluster.Spec.Topology.Variables,
	)
	if err == nil && workerConfig != nil {
		cfgsToValidate = append(cfgsToValidate, struct {
			cfg  *v1alpha1.KubeletConfiguration
			path string
		}{
			workerConfig.KubeletConfiguration,
			"workerConfig.kubeletConfiguration",
		})
	}

	for _, entry := range cfgsToValidate {
		if entry.cfg == nil {
			continue
		}

		if entry.cfg.AutomaticReservations != nil {
			if len(entry.cfg.SystemReserved) > 0 ||
				len(entry.cfg.KubeReserved) > 0 ||
				len(entry.cfg.EvictionHard) > 0 {
				return admission.Denied(fmt.Sprintf(
					"%s: automaticReservations cannot be combined with "+
						"systemReserved, kubeReserved, or evictionHard",
					entry.path,
				))
			}
		}

		if entry.cfg.CPUManagerPolicy != nil &&
			*entry.cfg.CPUManagerPolicy == v1alpha1.CPUManagerPolicyStatic {
			hasCPU := hasCPUReservation(entry.cfg.SystemReserved) ||
				hasCPUReservation(entry.cfg.KubeReserved)
			if !hasCPU {
				return admission.Denied(fmt.Sprintf(
					"%s: cpuManagerPolicy 'static' requires CPU "+
						"reservation in systemReserved or kubeReserved",
					entry.path,
				))
			}
		}

		if err := validateEvictionThresholds(
			entry.cfg.EvictionHard, entry.path+".evictionHard",
		); err != nil {
			return admission.Denied(err.Error())
		}
		if err := validateEvictionThresholds(
			entry.cfg.EvictionSoft, entry.path+".evictionSoft",
		); err != nil {
			return admission.Denied(err.Error())
		}
	}

	hasControlPlaneMaxParallel := clusterConfig.ControlPlane != nil &&
		clusterConfig.ControlPlane.KubeletConfiguration != nil &&
		clusterConfig.ControlPlane.KubeletConfiguration.MaxParallelImagePulls != nil
	hasWorkerMaxParallel := workerConfig != nil &&
		workerConfig.KubeletConfiguration != nil &&
		workerConfig.KubeletConfiguration.MaxParallelImagePulls != nil
	if clusterConfig.MaxParallelImagePullsPerNode != nil &&
		(hasControlPlaneMaxParallel || hasWorkerMaxParallel) {
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

func validateEvictionThresholds(thresholds map[string]string, fieldPath string) error {
	if thresholds == nil {
		return nil
	}
	for signal, val := range thresholds {
		if !evictionThresholdPattern.MatchString(val) {
			return fmt.Errorf(
				"%s: invalid eviction threshold value %q for signal %q: "+
					"must be a percentage or resource quantity",
				fieldPath, val, signal,
			)
		}
	}
	return nil
}
