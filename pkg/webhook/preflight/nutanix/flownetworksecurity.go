// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	CiliumConfigMapName            = "cilium-config"
	CiliumConfigMapNamespace       = "kube-system"
	CiliumLoadBalancerModeKey      = "bpf-lb-mode"
	CiliumLoadBalancerModeValueDSR = "dsr"

	CiliumForwardingModeAnnotationKey      = "service.cilium.io/forwarding-mode"
	CiliumForwardingModeAnnotationValueDSR = "dsr"
)

var FlowNetworkSecurityGroupVersion = schema.GroupVersion{
	Group:   "flow-container-security.nutanix.com",
	Version: "v1",
}

type FlowNetworkSecurityDSRToSNATChecker struct {
	kclient             ctrlclient.Client
	cluster             *clusterv1.Cluster
	clusterClientGetter remote.ClusterClientGetter
}

func (c *FlowNetworkSecurityDSRToSNATChecker) Name() string {
	return "NutanixFlowNetworkSecurityDSRToSNAT"
}

func (c *FlowNetworkSecurityDSRToSNATChecker) Run(ctx context.Context) preflight.CheckResult {
	remoteClient, err := c.clusterClientGetter(
		ctx,
		"",
		c.kclient,
		ctrlclient.ObjectKeyFromObject(c.cluster),
	)
	if err != nil {
		return preflight.CheckResult{
			Allowed:       false,
			InternalError: true,
			Causes: []preflight.Cause{
				{
					Message: fmt.Sprintf("error creating remote cluster client: %s", err),
				},
			},
		}
	}

	ciliumLoadBalancerMode, err := getCiliumLoadBalancerMode(ctx, remoteClient)
	if err != nil {
		return preflight.CheckResult{
			Allowed:       false,
			InternalError: true,
			Causes: []preflight.Cause{
				{
					Message: fmt.Sprintf("error checking Cilium loadBalancer mode configuration: %s", err),
				},
			},
		}
	}

	if ciliumLoadBalancerMode != CiliumLoadBalancerModeValueDSR {
		// The Cilium loadBalancer mode is not DSR.
		// Therefore, changing the mode from DSR to SNAT
		// will not impact Flow Network Security,
		// so the check passes.
		return preflight.CheckResult{
			Allowed: true,
		}
	}

	allKeys, err := getKeysOfLoadBalancerServicesInFNSEntityGroups(ctx, remoteClient)
	if err != nil {
		return preflight.CheckResult{
			Allowed:       false,
			InternalError: true,
			Causes: []preflight.Cause{
				{
					Message: fmt.Sprintf("error getting LoadBalancer Services in FNS entity groups: %s", err),
				},
			},
		}
	}

	missingCiliumDSRAnnotationsKeys := []ctrlclient.ObjectKey{}
	for _, service := range allKeys {
		hasCiliumDSRAnnotations, err := serviceHasCiliumDSRAnnotations(ctx, service, remoteClient)
		if err != nil {
			return preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: fmt.Sprintf(
							"error checking if LoadBalancer Service %s has Cilium DSR annotations: %s",
							service.String(),
							err,
						),
					},
				},
			}
		}
		if !hasCiliumDSRAnnotations {
			missingCiliumDSRAnnotationsKeys = append(missingCiliumDSRAnnotationsKeys, service)
		}
	}

	if len(missingCiliumDSRAnnotationsKeys) == 0 {
		// All LoadBalancer Services have the DSR annotation.
		// Therefore, changing the mode from DSR to SNAT
		// will not impact Flow Network Security,
		// so the check passes.
		return preflight.CheckResult{
			Allowed: true,
		}
	}

	return preflight.CheckResult{
		Allowed: false,
		Causes: []preflight.Cause{
			{
				Message: buildFailureMessage(missingCiliumDSRAnnotationsKeys),
			},
		},
	}
}

func newFlowNetworkSecurityDSRToSNATChecker(cd *checkDependencies) []preflight.Check {
	checks := []preflight.Check{}

	if cd == nil || cd.kclient == nil || cd.cluster == nil {
		return checks
	}

	if !ptr.Deref(cd.cluster.Status.Initialization.ControlPlaneInitialized, false) {
		// If the control plane is not initialized, then this cluster is not
		// currently enrolled in Flow Network Security, so the check is not relevant.
		return checks
	}

	return []preflight.Check{
		&FlowNetworkSecurityDSRToSNATChecker{
			kclient:             cd.kclient,
			cluster:             cd.cluster,
			clusterClientGetter: remote.NewClusterClient,
		},
	}
}

func getCiliumLoadBalancerMode(
	ctx context.Context,
	remoteClient ctrlclient.Client,
) (
	string,
	error,
) {
	ciliumConfigMap := &corev1.ConfigMap{}
	err := remoteClient.Get(ctx,
		ctrlclient.ObjectKey{
			Namespace: CiliumConfigMapNamespace,
			Name:      CiliumConfigMapName,
		},
		ciliumConfigMap,
	)
	if err != nil {
		return "", fmt.Errorf("error getting cilium config map: %w", err)
	}
	mode, ok := ciliumConfigMap.Data[CiliumLoadBalancerModeKey]
	if !ok {
		return "", fmt.Errorf("%s is not set in %s config map", CiliumLoadBalancerModeKey, CiliumConfigMapName)
	}
	return mode, nil
}

func getKeysOfLoadBalancerServicesInFNSEntityGroups(
	ctx context.Context,
	remoteClient ctrlclient.Client,
) (
	[]ctrlclient.ObjectKey,
	error,
) {
	entityGroups := &unstructured.UnstructuredList{}
	entityGroups.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   FlowNetworkSecurityGroupVersion.Group,
		Version: FlowNetworkSecurityGroupVersion.Version,
		Kind:    "EntityGroup",
	})
	err := remoteClient.List(ctx, entityGroups)
	if err != nil {
		return nil, fmt.Errorf("error listing entity groups: %w", err)
	}
	// When multiple EntityGroups reference the same LoadBalancer Service,
	// we only need to check the annotations of the Service once.
	// Therefore, we only record the ObjectKey of the Service once.
	uniqueObjectKeys := map[string]ctrlclient.ObjectKey{}
	for i := range entityGroups.Items {
		entityGroup := &entityGroups.Items[i]

		status, found, err := unstructured.NestedMap(entityGroup.Object, "status")
		if err != nil {
			return nil, fmt.Errorf(
				"getting status from entity group %q: %w",
				entityGroup.GetName(),
				err,
			)
		}
		if !found || len(status) == 0 {
			continue
		}

		serviceDetails, found, err := unstructured.NestedSlice(entityGroup.Object, "status", "serviceDetails")
		if err != nil {
			return nil, fmt.Errorf(
				"getting serviceDetails from entity group %q: %w",
				entityGroup.GetName(),
				err,
			)
		}
		if !found || len(serviceDetails) == 0 {
			continue
		}

		for _, entry := range serviceDetails {
			details, ok := entry.(map[string]any)
			if !ok {
				return nil, fmt.Errorf(
					"serviceDetails entry in entity group %q is %T, expected object",
					entityGroup.GetName(),
					entry,
				)
			}

			serviceType, _, err := unstructured.NestedString(details, "serviceType")
			if err != nil {
				return nil, fmt.Errorf(
					"getting serviceType from entity group %q: %w",
					entityGroup.GetName(),
					err,
				)
			}
			if corev1.ServiceType(serviceType) != corev1.ServiceTypeLoadBalancer {
				continue
			}

			serviceName, _, err := unstructured.NestedString(details, "serviceName")
			if err != nil {
				return nil, fmt.Errorf(
					"getting serviceName from entity group %q: %w",
					entityGroup.GetName(),
					err,
				)
			}
			serviceNamespace, _, err := unstructured.NestedString(details, "serviceNamespace")
			if err != nil {
				return nil, fmt.Errorf(
					"getting serviceNamespace from entity group %q: %w",
					entityGroup.GetName(),
					err,
				)
			}
			if serviceName == "" || serviceNamespace == "" {
				continue
			}

			key := ctrlclient.ObjectKey{
				Name:      serviceName,
				Namespace: serviceNamespace,
			}
			uniqueObjectKeys[key.String()] = key
		}
	}
	return slices.Collect(maps.Values(uniqueObjectKeys)), nil
}

func serviceHasCiliumDSRAnnotations(
	ctx context.Context,
	key ctrlclient.ObjectKey,
	remoteClient ctrlclient.Client,
) (bool, error) {
	service := &corev1.Service{}
	err := remoteClient.Get(ctx, key, service)
	if err != nil {
		return false, fmt.Errorf("error getting service %s: %w", key.String(), err)
	}
	val, ok := service.Annotations[CiliumForwardingModeAnnotationKey]
	if !ok || val != CiliumForwardingModeAnnotationValueDSR {
		return false, nil
	}
	return true, nil
}

func buildFailureMessage(missingCiliumDSRAnnotationsKeys []ctrlclient.ObjectKey) string {
	messageBuilder := strings.Builder{}
	//nolint:lll // This is a long message
	messageBuilder.WriteString(
		"This cluster is enrolled in Flow Network Security, and must use Direct Service Return (DSR) for these LoadBalancer Services: ",
	)

	// List the LoadBalancer Services that are missing the DSR annotation.
	serviceNamesAndNamespaces := make([]string, 0, len(missingCiliumDSRAnnotationsKeys))
	for _, service := range missingCiliumDSRAnnotationsKeys {
		serviceNamesAndNamespaces = append(
			serviceNamesAndNamespaces,
			fmt.Sprintf("%s/%s", service.Namespace, service.Name),
		)
	}
	messageBuilder.WriteString(strings.Join(serviceNamesAndNamespaces, ", "))
	messageBuilder.WriteString(".")

	// Explain the annotation to add.
	fmt.Fprintf(&messageBuilder, "Add the annotation '%s: %s' to the Service resources.",
		CiliumForwardingModeAnnotationKey,
		CiliumForwardingModeAnnotationValueDSR)
	return messageBuilder.String()
}
