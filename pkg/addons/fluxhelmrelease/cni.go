// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fluxhelmrelease

import (
	_ "embed" // embedding as []byte does not import the package.
	"fmt"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/fluxcd/pkg/apis/meta"
	fluxsourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// CNIForCluster returns a complete set of Cluster API objects to describe a CNI Configuration
// installable via Flux resources.
func CNIForCluster(cluster *clusterv1.Cluster) ([]unstructured.Unstructured, error) {
	vals, _ := yaml.YAMLToJSON([]byte(`
installation:
  cni:
    type: Calico
  # Configures Calico networking.
  calicoNetwork:
    # Note: The ipPools section cannot be modified post-install.
    ipPools:
    - blockSize: 26
      cidr: 192.168.0.0/16
      encapsulation: VXLANCrossSubnet
      natOutgoing: Enabled
      nodeSelector: all()
  nodeMetricsPort: 9091
  typhaMetricsPort: 9093
`))

	objs := []client.Object{
		&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "flux-helmrelease-addons",
			},
		},
		&fluxsourcev1beta2.HelmRepository{
			TypeMeta: metav1.TypeMeta{
				APIVersion: fluxsourcev1beta2.GroupVersion.String(),
				Kind:       fluxsourcev1beta2.HelmRepositoryKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "flux-helmrelease-addons",
				Name:      "projectcalico",
			},
			Spec: fluxsourcev1beta2.HelmRepositorySpec{
				URL: "https://docs.tigera.io/calico/charts",
			},
		},
		&fluxhelmv2beta1.HelmRelease{
			TypeMeta: metav1.TypeMeta{
				APIVersion: fluxhelmv2beta1.GroupVersion.String(),
				Kind:       fluxhelmv2beta1.HelmReleaseKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.Name + "-tigera-operator",
				Namespace: cluster.Namespace,
			},
			Spec: fluxhelmv2beta1.HelmReleaseSpec{
				KubeConfig: &fluxhelmv2beta1.KubeConfig{
					SecretRef: meta.SecretKeyReference{
						Name: fmt.Sprintf("%s-kubeconfig", cluster.Name),
						Key:  "value",
					},
				},
				TargetNamespace: "tigera-operator",
				ReleaseName:     "tigera-operator",
				Chart: fluxhelmv2beta1.HelmChartTemplate{
					Spec: fluxhelmv2beta1.HelmChartTemplateSpec{
						SourceRef: fluxhelmv2beta1.CrossNamespaceObjectReference{
							APIVersion: fluxsourcev1beta2.GroupVersion.String(),
							Kind:       fluxsourcev1beta2.HelmRepositoryKind,
							Namespace:  "flux-helmrelease-addons",
							Name:       "projectcalico",
						},
						Chart:   "tigera-operator",
						Version: " v3.25.0",
					},
				},
				Values: &apiextensionsv1.JSON{Raw: vals},
				Install: &fluxhelmv2beta1.Install{
					CreateNamespace: true,
					CRDs:            fluxhelmv2beta1.CreateReplace,
					Remediation: &fluxhelmv2beta1.InstallRemediation{
						Retries: 30,
					},
				},
				Upgrade: &fluxhelmv2beta1.Upgrade{
					CRDs: fluxhelmv2beta1.CreateReplace,
					Remediation: &fluxhelmv2beta1.UpgradeRemediation{
						Retries: 30,
					},
				},
			},
		},
	}

	unstrObjs := make([]unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, err
		}
		unstrObjs = append(unstrObjs, unstructured.Unstructured{Object: unstrObj})
	}

	return unstrObjs, nil
}
