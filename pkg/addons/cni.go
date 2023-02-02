// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (

	// embedding as []byte does not import the package.
	_ "embed"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	//go:embed templates/cni/calico-cni-installation-crs.yaml
	calicoInstallationCRSTmpl []byte

	//go:embed templates/cni/tigera-operator-configmap.yaml
	tigeraOperatorConfigMapTmpl []byte

	//go:embed templates/cni/docker-calico-cni-installation-configmap.yaml
	calicoInstallationConfigMapDockerTmpl []byte
)

// CNIForCluster returns a complete set of Cluster API objects to describe a CNI Configuration
// Including a ClusterResourceSet and referenced ConfigMaps.
func CNIForCluster(cluster *clusterv1.Cluster) ([]unstructured.Unstructured, error) {
	return crsObjsFromTemplates(
		cluster.Namespace,
		calicoInstallationCRSTmpl,
		calicoInstallationConfigMapDockerTmpl,
		tigeraOperatorConfigMapTmpl,
	)
}
