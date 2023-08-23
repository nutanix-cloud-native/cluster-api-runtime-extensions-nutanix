// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"bytes"
	"context"
	"fmt"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructuredscheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/cni"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/k8s/parser"
)

const (
	CNILabelValue = "calico"
)

type CalicoCNIConfig struct {
	defaultsNamespace string

	defaultTigeraOperatorConfigMapName        string
	defaultProviderInstallationConfigMapNames map[string]string
}

func (c *CalicoCNIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultsNamespace,
		prefix+".defaultsNamespace",
		corev1.NamespaceDefault,
		"name of the ConfigMap used to deploy Tigera Operator",
	)

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
			"DockerCluster": "calico-cni-installation-dockercluster",
		},
		"map of provider cluster implementation type to default installation ConfigMap name",
	)
}

type CalicoCNI struct {
	client ctrlclient.Client
	config CalicoCNIConfig
}

var (
	_ handlers.NamedHandler                                 = &CalicoCNI{}
	_ handlers.AfterControlPlaneInitializedLifecycleHandler = &CalicoCNI{}

	calicoInstallationGK = schema.GroupKind{Group: "operator.tigera.io", Kind: "Installation"}
)

func New(c ctrlclient.Client, cfg CalicoCNIConfig) *CalicoCNI {
	return &CalicoCNI{
		client: c,
		config: cfg,
	}
}

func (s *CalicoCNI) Name() string {
	return "CalicoCNI"
}

func (s *CalicoCNI) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(&req.Cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	// Safe to set this to success response before we actually do the apply as the error handling
	// will update for failure response properly.
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)

	if v, ok := req.Cluster.GetLabels()[cni.CNIProviderLabelKey]; !ok || v != CNILabelValue {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping Calico CNI handler, cluster does not specify %q as value of CNI provider label %q",
				CNILabelValue,
				cni.CNIProviderLabelKey,
			),
		)
		return
	}

	defaultInstallationConfigMapName, ok := s.config.defaultProviderInstallationConfigMapNames[req.Cluster.Spec.InfrastructureRef.Kind] //nolint:lll // Just a long line...
	if !ok {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping Calico CNI handler, no default installation ConfigMap configured for infrastructure provider %q",
				req.Cluster.Spec.InfrastructureRef.Kind,
			),
		)
		return
	}

	log.Info("Ensuring Tigera CRS and manifests ConfigMap exist in cluster namespace")
	defaultTigeraOperatorConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.config.defaultsNamespace,
			Name:      s.config.defaultTigeraOperatorConfigMapName,
		},
	}
	defaultTigeraOperatorConfigMapObjName := ctrlclient.ObjectKeyFromObject(
		defaultTigeraOperatorConfigMap,
	)
	err := s.client.Get(ctx, defaultTigeraOperatorConfigMapObjName, defaultTigeraOperatorConfigMap)
	if err != nil {
		log.Error(
			err,
			fmt.Sprintf(
				"failed to retrieve default Tigera Operator ConfigMap %q",
				defaultTigeraOperatorConfigMapObjName,
			),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to retrieve default Tigera Operator ConfigMap %q: %v",
				defaultTigeraOperatorConfigMapObjName,
				err,
			),
		)
		return
	}
	tigeraObjs := generateTigeraOperatorCRS(defaultTigeraOperatorConfigMap, &req.Cluster)
	if err := client.ServerSideApply(ctx, s.client, tigeraObjs...); err != nil {
		log.Error(err, "failed to apply Tigera ClusterResourceSet")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to apply Tigera ClusterResourceSet: %v", err))
		return
	}

	log.Info("Ensuring Calico installation CRS and ConfigMap exist in cluster namespace")
	defaultInstallationConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.config.defaultsNamespace,
			Name:      defaultInstallationConfigMapName,
		},
	}
	defaultInstallationConfigMapObjName := ctrlclient.ObjectKeyFromObject(
		defaultInstallationConfigMap,
	)
	err = s.client.Get(ctx, defaultInstallationConfigMapObjName, defaultInstallationConfigMap)
	if err != nil {
		log.Error(
			err,
			fmt.Sprintf(
				"failed to retrieve default default installation ConfigMap %q",
				defaultInstallationConfigMapObjName,
			),
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"failed to retrieve default default installation ConfigMap %q: %v",
				defaultInstallationConfigMapObjName,
				err,
			),
		)
		return
	}
	calicoCNIObjs, err := generateProviderCNICRS(
		defaultInstallationConfigMap,
		&req.Cluster,
		s.client.Scheme(),
	)
	if err != nil {
		log.Error(err, "failed to generate provider CNI CRS")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to generate provider CNI CRS: %v", err))
		return
	}

	if err := client.ServerSideApply(ctx, s.client, calicoCNIObjs...); err != nil {
		log.Error(err, "failed to apply CNI installation ClusterResourceSet")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to apply CNI installation ClusterResourceSet: %v", err))
	}
}

func generateTigeraOperatorCRS(
	defaultTigeraOperatorConfigMap *corev1.ConfigMap, cluster *capiv1.Cluster,
) []ctrlclient.Object {
	namespacedTigeraConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.GetNamespace(),
			Name:      defaultTigeraOperatorConfigMap.Name,
		},
		Data:       defaultTigeraOperatorConfigMap.Data,
		BinaryData: defaultTigeraOperatorConfigMap.BinaryData,
	}

	tigeraCRS := &crsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: crsv1.GroupVersion.String(),
			Kind:       "ClusterResourceSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.GetNamespace(),
			Name:      namespacedTigeraConfigMap.GetName(),
		},
		Spec: crsv1.ClusterResourceSetSpec{
			Resources: []crsv1.ResourceRef{{
				Kind: string(crsv1.ConfigMapClusterResourceSetResourceKind),
				Name: namespacedTigeraConfigMap.GetName(),
			}},
			Strategy: string(crsv1.ClusterResourceSetStrategyReconcile),
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{cni.CNIProviderLabelKey: CNILabelValue},
			},
		},
	}

	return []ctrlclient.Object{namespacedTigeraConfigMap, tigeraCRS}
}

func generateProviderCNICRS(
	installationConfigMap *corev1.ConfigMap, cluster *capiv1.Cluster, scheme *runtime.Scheme,
) ([]ctrlclient.Object, error) {
	defaultManifestStrings := make([]string, 0, len(installationConfigMap.Data))
	for _, v := range installationConfigMap.Data {
		defaultManifestStrings = append(defaultManifestStrings, v)
	}
	parsed, err := parser.StringsToUnstructured(defaultManifestStrings...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded manifests: %w", err)
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
		Data: make(map[string]string, 1),
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

	cm.Data["manifests"] = b.String()

	if err := controllerutil.SetOwnerReference(cluster, cm, scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	crs := &crsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: crsv1.GroupVersion.String(),
			Kind:       "ClusterResourceSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.GetNamespace(),
			Name:      cm.GetName(),
		},
		Spec: crsv1.ClusterResourceSetSpec{
			Resources: []crsv1.ResourceRef{{
				Kind: string(crsv1.ConfigMapClusterResourceSetResourceKind),
				Name: cm.GetName(),
			}},
			Strategy: string(crsv1.ClusterResourceSetStrategyReconcile),
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{capiv1.ClusterNameLabel: cluster.GetName()},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(cluster, crs, scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return []ctrlclient.Object{cm, crs}, nil
}
