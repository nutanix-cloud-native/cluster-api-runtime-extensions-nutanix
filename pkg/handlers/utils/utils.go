// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

type EnsureCRSForClusterFromObjectsOptions struct {
	// OwnerCluster holds the owning cluster for the ClusterResourceSet.
	// This allows setting the owner to something other than the workload cluster, which is
	// needed specifically for the ClusterAutoscaler addon which can be deployed in a different
	// namespace to the target cluster as it must exist in the management cluster.
	OwnerCluster *clusterv1.Cluster
}

func DefaultEnsureCRSForClusterFromObjectsOptions() EnsureCRSForClusterFromObjectsOptions {
	return EnsureCRSForClusterFromObjectsOptions{}
}

func (o EnsureCRSForClusterFromObjectsOptions) WithOwnerCluster(
	cluster *clusterv1.Cluster,
) EnsureCRSForClusterFromObjectsOptions {
	o.OwnerCluster = cluster
	return o
}

func EnsureCRSForClusterFromObjects(
	ctx context.Context,
	crsName string,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	opts EnsureCRSForClusterFromObjectsOptions,
	objects ...runtime.Object,
) error {
	resources := make([]crsv1.ResourceRef, 0, len(objects))
	for _, obj := range objects {
		var name string
		var kind crsv1.ClusterResourceSetResourceKind
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			sec, secOk := obj.(*corev1.Secret)
			if !secOk {
				return fmt.Errorf(
					"cannot create ClusterResourceSet with obj %v only secrets and configmaps are supported",
					obj,
				)
			}
			name = sec.Name
			kind = crsv1.SecretClusterResourceSetResourceKind
		} else {
			name = cm.Name
			kind = crsv1.ConfigMapClusterResourceSetResourceKind
		}
		resources = append(resources, crsv1.ResourceRef{
			Name: name,
			Kind: string(kind),
		})
	}

	crs := &crsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: crsv1.GroupVersion.String(),
			Kind:       "ClusterResourceSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      crsName,
		},
		Spec: crsv1.ClusterResourceSetSpec{
			Resources: resources,
			Strategy:  string(crsv1.ClusterResourceSetStrategyReconcile),
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
		},
	}

	ownerCluster := cluster
	if opts.OwnerCluster != nil {
		ownerCluster = opts.OwnerCluster
	}
	if err := controllerutil.SetOwnerReference(ownerCluster, crs, c.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	err := client.ServerSideApply(ctx, c, crs, client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("failed to server side apply: %w", err)
	}

	return nil
}

// EnsureNamespaceWithName will create the namespace with the specified name if it does not exist.
func EnsureNamespaceWithName(ctx context.Context, c ctrlclient.Client, name string) error {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	return EnsureNamespace(ctx, c, ns)
}

// EnsureNamespaceWithMetadata will create the namespace with the specified name,
// labels, and/or annotations, if it does not exist.
func EnsureNamespaceWithMetadata(ctx context.Context,
	c ctrlclient.Client,
	name string,
	labels, annotations map[string]string,
) error {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
			Labels:      labels,
		},
	}

	return EnsureNamespace(ctx, c, ns)
}

// EnsureNamespace will create the namespace if it does not exist.
func EnsureNamespace(ctx context.Context, c ctrlclient.Client, ns *corev1.Namespace) error {
	if ns.APIVersion == "" {
		ns.APIVersion = corev1.SchemeGroupVersion.String()
	}
	if ns.Kind == "" {
		ns.Kind = "Namespace"
	}
	err := client.ServerSideApply(ctx, c, ns)
	if err != nil {
		return fmt.Errorf("failed to server side apply: %w", err)
	}

	return nil
}

func RetrieveValuesTemplateConfigMap(
	ctx context.Context,
	c ctrlclient.Client,
	configMapName,
	namespace string,
) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      configMapName,
		},
	}
	configMapObjName := ctrlclient.ObjectKeyFromObject(
		configMap,
	)
	err := c.Get(ctx, configMapObjName, configMap)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to retrieve installation values template ConfigMap %q: %w",
			configMapObjName,
			err,
		)
	}
	return configMap, nil
}

func RetrieveValuesTemplate(
	ctx context.Context,
	c ctrlclient.Client,
	configMapName,
	namespace string,
) (string, error) {
	configMap, err := RetrieveValuesTemplateConfigMap(ctx, c, configMapName, namespace)
	if err != nil {
		return "", err
	}
	return configMap.Data["values.yaml"], nil
}

func SetTLSConfigForHelmChartProxyIfNeeded(hcp *caaphv1.HelmChartProxy) {
	// this is set as an environment variable from the downward API on deployment
	deploymentNS := os.Getenv("POD_NAMESPACE")
	if deploymentNS == "" {
		deploymentNS = metav1.NamespaceDefault
	}
	if strings.Contains(hcp.Spec.RepoURL, "helm-repository") {
		hcp.Spec.TLSConfig = &caaphv1.TLSConfig{
			CASecretRef: &corev1.SecretReference{
				Name:      "helm-repository-tls",
				Namespace: deploymentNS,
			},
		}
	}
}
