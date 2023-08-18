// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"

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

type CalicoCNI struct {
	client ctrlclient.Client
}

var (
	_ handlers.NamedHandler                                 = &CalicoCNI{}
	_ handlers.AfterControlPlaneInitializedLifecycleHandler = &CalicoCNI{}

	//go:embed manifests/tigera-operator-configmap.yaml
	tigeraConfigMapBytes []byte

	// Only need to parse this on start-up and only once. If this isn't a valid configmap then this
	// will panic on startup, and exit early.
	tigeraConfigMap = parser.MustParseToObjects[*corev1.ConfigMap](tigeraConfigMapBytes)[0].(*corev1.ConfigMap)

	//go:embed manifests/docker
	dockerCNIManifests embed.FS

	providerManifestsFS = map[string]embed.FS{
		"DockerCluster": dockerCNIManifests,
	}

	calicoInstallationGK = schema.GroupKind{Group: "operator.tigera.io", Kind: "Installation"}
)

func New(c ctrlclient.Client) *CalicoCNI {
	return &CalicoCNI{
		client: c,
	}
}

func (s *CalicoCNI) Name() string {
	return "calico-cni"
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

	manifestsFS, ok := providerManifestsFS[req.Cluster.Spec.InfrastructureRef.Kind]
	if !ok {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping Calico CNI handler, no default configured for infrastructure provider %q",
				req.Cluster.Spec.InfrastructureRef.Kind,
			),
		)
		return
	}

	log.Info("Ensuring Tigera CRS and manifests ConfigMap exist in cluster namespace")
	tigeraObjs := generateTigeraOperatorCRS(&req.Cluster)
	if err := client.ServerSideApply(ctx, s.client, tigeraObjs...); err != nil {
		log.Error(err, "failed to apply Tigera ClusterResourceSet")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to apply Tigera ClusterResourceSet: %v", err))
		return
	}

	log.Info("Ensuring Calico installation CRS and ConfigMap exist in cluster namespace")
	calicoCNIObjs, err := generateProviderCNICRS(manifestsFS, &req.Cluster, s.client.Scheme())
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

func generateTigeraOperatorCRS(cluster *capiv1.Cluster) []ctrlclient.Object {
	// Set the namespace on the tigera configmap to apply by deep copying and then mutating.
	namespacedTigeraConfigMap := &corev1.ConfigMap{}
	tigeraConfigMap.DeepCopyInto(namespacedTigeraConfigMap)
	namespacedTigeraConfigMap.SetNamespace(cluster.GetNamespace())

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

func generateProviderCNICRS(manifestsFS fs.FS, cluster *capiv1.Cluster, scheme *runtime.Scheme) ([]ctrlclient.Object, error) {
	readers, cleanup, err := readersForManifestsInFS(manifestsFS)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded manifests: %w", err)
	}
	defer func() { _ = cleanup() }()

	parsed, err := parser.ReadersToUnstructured(readers...)
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

	podSubnet, podSubnetSpecified := cluster.GetAnnotations()[cni.PodSubnetAnnotationKey]

	var b bytes.Buffer

	for _, o := range parsed {
		if podSubnetSpecified &&
			podSubnet != "" &&
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

func readersForManifestsInFS(
	manifestsFS fs.FS,
) (readers []io.Reader, cleanup func() error, err error) {
	var manifestFiles []string
	err = fs.WalkDir(manifestsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		manifestFiles = append(manifestFiles, path)

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to walk embedded filesystem: %w", err)
	}

	readers = make([]io.Reader, 0, len(manifestFiles))
	for _, mf := range manifestFiles {
		f, err := manifestsFS.Open(mf)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open embedded manifests: %w", err)
		}
		readers = append(readers, f)
	}

	cleanup = func() error {
		for _, r := range readers {
			if err := r.(io.ReadCloser).Close(); err != nil {
				return err
			}
		}

		return nil
	}

	return readers, cleanup, nil
}
