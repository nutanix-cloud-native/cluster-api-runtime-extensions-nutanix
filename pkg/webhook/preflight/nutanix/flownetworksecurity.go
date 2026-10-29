// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	CiliumConfigMapName             = "cilium-config"
	CiliumConfigMapNamespace        = "kube-system"
	CiliumLoadBalancerModeKey       = "bpf-lb-mode"
	CiliumLoadBalancerModeValueDSR  = "dsr"
	CiliumLoadBalancerModeValueSNAT = "snat"

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

const flowNetworkSecurityDSRToSNATCheckName = "NutanixFlowNetworkSecurityDSRToSNAT"

func (c *FlowNetworkSecurityDSRToSNATChecker) Name() string {
	return flowNetworkSecurityDSRToSNATCheckName
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

	allKeys, err := getKeysOfNodePortAndLoadBalancerServicesInFNSEntityGroups(ctx, remoteClient)
	if err != nil {
		return preflight.CheckResult{
			Allowed:       false,
			InternalError: true,
			Causes: []preflight.Cause{
				{
					Message: fmt.Sprintf(
						"error getting NodePort and LoadBalancer Services in FNS entity groups: %s",
						err,
					),
				},
			},
		}
	}

	requiresCiliumDSRAnnotationsKeys := []ctrlclient.ObjectKey{}
	for _, service := range allKeys {
		requiresCiliumDSRAnnotation, err := serviceRequiresCiliumDSRAnnotation(ctx, service, remoteClient)
		if err != nil {
			return preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: fmt.Sprintf(
							"error checking if Service %s requires Cilium DSR annotation: %s",
							service.String(),
							err,
						),
					},
				},
			}
		}
		if requiresCiliumDSRAnnotation {
			requiresCiliumDSRAnnotationsKeys = append(requiresCiliumDSRAnnotationsKeys, service)
		}
	}

	if len(requiresCiliumDSRAnnotationsKeys) == 0 {
		// All NodePort and LoadBalancer Services have the DSR annotation.
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
				Message: buildFailureMessage(requiresCiliumDSRAnnotationsKeys),
			},
		},
	}
}

func newFlowNetworkSecurityDSRToSNATChecker(cd *checkDependencies) []preflight.Check {
	checks := []preflight.Check{}

	if cd == nil || cd.kclient == nil || cd.cluster == nil {
		return checks
	}

	if cd.oldCluster == nil {
		// This cluster is being created. This check is only relevant for updates to existing clusters.
		return checks
	}

	if cd.oldCluster.Spec.Topology.Version == cd.cluster.Spec.Topology.Version {
		// The CNI configuration only changes when the cluster version changes.
		// The cluster version has not changed, so this check is not relevant.
		return checks
	}

	cniProvider, err := getCNIProvider(cd.cluster)
	if err != nil {
		return []preflight.Check{
			&flowNetworkSecurityDSRToSNATErrorCheck{
				err: fmt.Errorf(
					"failed to determine the CNI provider of the cluster, which is needed to decide whether "+
						"the Cilium loadBalancer mode change affects Flow Network Security: %w",
					err,
				),
			},
		}
	}
	if cniProvider != carenv1.CNIProviderCilium {
		// This cluster is not using Cilium, so the check is not relevant.
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

// flowNetworkSecurityDSRToSNATErrorCheck shares the name of FlowNetworkSecurityDSRToSNATChecker, so that
// the same skip annotation applies to both.
type flowNetworkSecurityDSRToSNATErrorCheck struct {
	err error
}

func (c *flowNetworkSecurityDSRToSNATErrorCheck) Name() string {
	return flowNetworkSecurityDSRToSNATCheckName
}

func (c *flowNetworkSecurityDSRToSNATErrorCheck) Run(_ context.Context) preflight.CheckResult {
	return preflight.CheckResult{
		Allowed:       false,
		InternalError: true,
		Causes: []preflight.Cause{
			{
				Message: c.err.Error(),
			},
		},
	}
}

func getCNIProvider(cluster *clusterv1.Cluster) (string, error) {
	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)

	cniVar, err := variables.Get[carenv1.NutanixCNI](
		varMap,
		carenv1.ClusterConfigVariableName,
		"addons",
		carenv1.CNIVariableName,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			return "", nil // The CNI provider is not specified, so we assume it is not Cilium.
		}
		return "", fmt.Errorf("error getting CNI provider from Cluster Addons configuration: %w", err)
	}
	return cniVar.Provider, nil
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
		// This check only runs if the CNI provider is Cilium. Therefore, the
		// ConfigMap must exist. A missing ConfigMap means an error, not that
		// the check does not apply
		return "", fmt.Errorf("error getting cilium config map: %w", err)
	}
	mode, ok := ciliumConfigMap.Data[CiliumLoadBalancerModeKey]
	if !ok {
		// SNAT is the mode Cilium uses when bpf-lb-mode is not set.
		return CiliumLoadBalancerModeValueSNAT, nil
	}
	return mode, nil
}

func getKeysOfNodePortAndLoadBalancerServicesInFNSEntityGroups(
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
		if meta.IsNoMatchError(err) {
			// The EntityGroup CRD is not installed, so the cluster is not enrolled in Flow Network Security.
			return nil, nil
		}
		return nil, fmt.Errorf("error listing entity groups: %w", err)
	}
	// When multiple EntityGroups reference the same Service,
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
			if !isServiceTypeAffectedByCiliumLoadBalancerMode(corev1.ServiceType(serviceType)) {
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
	return slices.SortedFunc(maps.Values(uniqueObjectKeys), func(a, b ctrlclient.ObjectKey) int {
		return strings.Compare(a.String(), b.String())
	}), nil
}

// serviceRequiresCiliumDSRAnnotation reads the Service, rather than trusting the EntityGroup status,
// because the status can be out of date.
func serviceRequiresCiliumDSRAnnotation(
	ctx context.Context,
	key ctrlclient.ObjectKey,
	remoteClient ctrlclient.Client,
) (bool, error) {
	service := &corev1.Service{}
	err := remoteClient.Get(ctx, key, service)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("error getting service %s: %w", key.String(), err)
	}
	if !isServiceTypeAffectedByCiliumLoadBalancerMode(service.Spec.Type) {
		return false, nil
	}
	return service.Annotations[CiliumForwardingModeAnnotationKey] != CiliumForwardingModeAnnotationValueDSR, nil
}

// isServiceTypeAffectedByCiliumLoadBalancerMode returns true for the Service types that Cilium
// forwards using its loadBalancer mode.
func isServiceTypeAffectedByCiliumLoadBalancerMode(serviceType corev1.ServiceType) bool {
	return serviceType == corev1.ServiceTypeNodePort || serviceType == corev1.ServiceTypeLoadBalancer
}

func buildFailureMessage(requiresCiliumDSRAnnotationsKeys []ctrlclient.ObjectKey) string {
	messageBuilder := strings.Builder{}
	//nolint:lll // This is a long message
	messageBuilder.WriteString(
		"This cluster is enrolled in Flow Network Security, and must use Direct Server Return (DSR) for these NodePort and LoadBalancer Services: ",
	)

	// List the NodePort and LoadBalancer Services that require the DSR annotation.
	serviceNamesAndNamespaces := make([]string, 0, len(requiresCiliumDSRAnnotationsKeys))
	for _, service := range requiresCiliumDSRAnnotationsKeys {
		serviceNamesAndNamespaces = append(
			serviceNamesAndNamespaces,
			fmt.Sprintf("%s/%s", service.Namespace, service.Name),
		)
	}
	messageBuilder.WriteString(strings.Join(serviceNamesAndNamespaces, ", "))
	messageBuilder.WriteString(". ")

	// Explain the annotation to add.
	fmt.Fprintf(&messageBuilder, "Add the annotation '%s: %s' to the above Service resources.",
		CiliumForwardingModeAnnotationKey,
		CiliumForwardingModeAnnotationValueDSR)
	return messageBuilder.String()
}
