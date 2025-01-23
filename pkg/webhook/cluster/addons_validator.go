// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type addonsValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewAddonsValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *addonsValidator {
	return &addonsValidator{
		client:  client,
		decoder: decoder,
	}
}

func (a *addonsValidator) Validator() admission.HandlerFunc {
	return a.validate
}

func (a *addonsValidator) validate(
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

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w",
				v1alpha1.ClusterConfigVariableName,
				err).Error(),
		)
	}

	if clusterConfig.Addons != nil && clusterConfig.Addons.HelmChartConfig != nil {
		// Check if custom helm chart ConfigMap is provided
		if err := validateCustomHelmChartConfigMapExists(
			ctx,
			a.client,
			clusterConfig.Addons.HelmChartConfig.ConfigMapRef.Name,
			cluster.Namespace,
		); err != nil {
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}

func validateCustomHelmChartConfigMapExists(
	ctx context.Context,
	client ctrlclient.Client,
	name string,
	namespace string,
) error {
	configMap := &corev1.ConfigMap{}
	err := client.Get(ctx, ctrlclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf(
				"HelmChart ConfigMap %q referenced in the cluster variables not found: %w",
				name,
				err,
			)
		}
		return fmt.Errorf("failed to get HelmChart ConfigMap %q: %w", name, err)
	}
	return nil
}
