// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"bytes"
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructuredscheme"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/parser"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/cni"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
)

type crsConfig struct {
	defaultTigeraOperatorConfigMapName        string
	defaultProviderInstallationConfigMapNames map[string]string
}

func (c *crsConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultTigeraOperatorConfigMapName,
		prefix+".default-tigera-operator-configmap-name",
		"tigera-operator",
		"name of the ConfigMap used to deploy Tigera Operator",
	)
	flags.StringToStringVar(
		&c.defaultProviderInstallationConfigMapNames,
		prefix+".default-provider-installation-configmap-names",
		map[string]string{
			"DockerCluster":  "calico-cni-crs-installation-dockercluster",
			"AWSCluster":     "calico-cni-crs-installation-awscluster",
			"NutanixCluster": "calico-cni-crs-installation-nutanixcluster",
		},
		"map of provider cluster implementation type to default installation ConfigMap name",
	)
}

type crsStrategy struct {
	config crsConfig

	client ctrlclient.Client
}

func (s crsStrategy) apply(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	defaultsNamespace string,
	log logr.Logger,
) error {
	infraKind := req.Cluster.Spec.InfrastructureRef.Kind
	defaultInstallationConfigMapName, ok := s.config.defaultProviderInstallationConfigMapNames[infraKind]
	if !ok {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping Calico CNI handler, no default installation ConfigMap configured for infrastructure provider %q",
				req.Cluster.Spec.InfrastructureRef.Kind,
			),
		)
		return nil
	}

	log.Info("Ensuring Tigera manifests ConfigMap exist in cluster namespace")
	tigeraCM, err := s.ensureTigeraOperatorConfigMap(ctx, &req.Cluster, defaultsNamespace)
	if err != nil {
		log.Error(
			err,
			"failed to ensure Tigera Operator manifests ConfigMap exists in cluster namespace",
		)
		return fmt.Errorf(
			"failed to ensure Tigera Operator manifests ConfigMap exists in cluster namespace: %w",
			err,
		)
	}

	log.Info("Ensuring Calico installation CRS and ConfigMap exist for cluster")
	if err := s.ensureCNICRSForCluster(
		ctx,
		&req.Cluster,
		defaultsNamespace,
		defaultInstallationConfigMapName,
		tigeraCM,
	); err != nil {
		log.Error(
			err,
			"failed to ensure Calico installation manifests ConfigMap and ClusterResourceSet exist in cluster namespace",
		)
		return fmt.Errorf(
			"failed to ensure Tigera Operator manifests ConfigMap exists in cluster namespace: %w",
			err,
		)
	}

	return nil
}

func (s crsStrategy) ensureCNICRSForCluster(
	ctx context.Context,
	cluster *capiv1.Cluster,
	defaultsNamespace string,
	defaultInstallationConfigMapName string,
	tigeraConfigMap *corev1.ConfigMap,
) error {
	defaultInstallationConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultsNamespace,
			Name:      defaultInstallationConfigMapName,
		},
	}
	defaultInstallationConfigMapObjName := ctrlclient.ObjectKeyFromObject(
		defaultInstallationConfigMap,
	)
	err := s.client.Get(ctx, defaultInstallationConfigMapObjName, defaultInstallationConfigMap)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve default installation ConfigMap %q: %w",
			defaultInstallationConfigMapObjName,
			err,
		)
	}

	cm, err := generateProviderCNIManifestsConfigMap(
		defaultInstallationConfigMap,
		cluster,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to generate Calico provider CNI manifests ConfigMap: %w",
			err,
		)
	}

	if err := client.ServerSideApply(ctx, s.client, cm); err != nil {
		return fmt.Errorf(
			"failed to apply Calico CNI installation manifests ConfigMap: %w",
			err,
		)
	}

	if err := utils.EnsureCRSForClusterFromObjects(ctx, cm.Name, s.client, cluster, tigeraConfigMap, cm); err != nil {
		return fmt.Errorf(
			"failed to apply Calico CNI installation ClusterResourceSet: %w",
			err,
		)
	}

	return nil
}

func (s crsStrategy) ensureTigeraOperatorConfigMap(
	ctx context.Context,
	cluster *capiv1.Cluster,
	defaultsNamespace string,
) (*corev1.ConfigMap, error) {
	defaultTigeraOperatorConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultsNamespace,
			Name:      s.config.defaultTigeraOperatorConfigMapName,
		},
	}
	defaultTigeraOperatorConfigMapObjName := ctrlclient.ObjectKeyFromObject(
		defaultTigeraOperatorConfigMap,
	)
	err := s.client.Get(ctx, defaultTigeraOperatorConfigMapObjName, defaultTigeraOperatorConfigMap)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to retrieve default Tigera Operator manifests ConfigMap %q: %w",
			defaultTigeraOperatorConfigMapObjName,
			err,
		)
	}

	tigeraConfigMap := generateTigeraOperatorConfigMap(defaultTigeraOperatorConfigMap, cluster)
	if err := client.ServerSideApply(ctx, s.client, tigeraConfigMap); err != nil {
		return nil, fmt.Errorf(
			"failed to apply Tigera Operator manifests ConfigMap: %w",
			err,
		)
	}

	return tigeraConfigMap, nil
}

func generateTigeraOperatorConfigMap(
	defaultTigeraOperatorConfigMap *corev1.ConfigMap, cluster *capiv1.Cluster,
) *corev1.ConfigMap {
	namespacedTigeraConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf("%s-%s", defaultTigeraOperatorConfigMap.Name, cluster.Name),
		},
		Data:       defaultTigeraOperatorConfigMap.Data,
		BinaryData: defaultTigeraOperatorConfigMap.BinaryData,
	}

	return namespacedTigeraConfigMap
}

func generateProviderCNIManifestsConfigMap(
	installationConfigMap *corev1.ConfigMap,
	cluster *capiv1.Cluster,
) (*corev1.ConfigMap, error) {
	defaultManifestStrings := make([]string, 0, len(installationConfigMap.Data))
	for _, v := range installationConfigMap.Data {
		defaultManifestStrings = append(defaultManifestStrings, v)
	}
	parsed, err := parser.StringsToUnstructured(defaultManifestStrings...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded manifests: %w", err)
	}

	yamlSerializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		unstructuredscheme.NewUnstructuredCreator(),
		unstructuredscheme.NewUnstructuredObjectTyper(),
		json.SerializerOptions{
			Yaml:   true,
			Strict: true,
		},
	)

	podSubnet, err := cni.PodCIDR(cluster)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer

	for _, o := range parsed {
		calicoInstallationGK := schema.GroupKind{Group: "operator.tigera.io", Kind: "Installation"}
		if podSubnet != "" &&
			o.GetObjectKind().GroupVersionKind().GroupKind() == calicoInstallationGK {
			obj := o.(*unstructured.Unstructured).Object

			ipPoolsRef, exists, err := unstructured.NestedFieldNoCopy(
				obj,
				"spec", "calicoNetwork", "ipPools",
			)
			if err != nil {
				return nil, fmt.Errorf("failed to get ipPools from unstructured object: %w", err)
			}
			if !exists {
				return nil, fmt.Errorf("missing ipPools in unstructured object")
			}

			ipPools := ipPoolsRef.([]interface{})

			err = unstructured.SetNestedField(
				ipPools[0].(map[string]interface{}),
				podSubnet,
				"cidr",
			)
			if err != nil {
				return nil, fmt.Errorf("failed to set default pod subnet: %w", err)
			}

			err = unstructured.SetNestedSlice(obj, ipPools, "spec", "calicoNetwork", "ipPools")
			if err != nil {
				return nil, fmt.Errorf("failed to update ipPools: %w", err)
			}
		}

		if err := yamlSerializer.Encode(o, &b); err != nil {
			return nil, fmt.Errorf("failed to serialize manifests: %w", err)
		}

		_, _ = b.WriteString("\n---\n")
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "calico-cni-installation-" + cluster.Name,
		},
		Data: map[string]string{
			"manifests": b.String(),
		},
	}

	return cm, nil
}
