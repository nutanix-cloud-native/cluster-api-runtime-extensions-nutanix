// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package variables holds the variable schemas served by this handler version, built from its own CRD snapshot so
// that later API changes do not change them.
package variables

import (
	"embed"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

//go:embed crds/*.yaml
var crds embed.FS

var (
	AWSClusterConfig     = schema("awsclusterconfigs")
	AWSWorkerConfig      = schema("awsworkernodeconfigs")
	DockerClusterConfig  = schema("dockerclusterconfigs")
	DockerWorkerConfig   = schema("dockerworkernodeconfigs")
	EKSClusterConfig     = schema("eksclusterconfigs")
	EKSWorkerConfig      = schema("eksworkernodeconfigs")
	GenericClusterConfig = schema("genericclusterconfigs")
	NutanixClusterConfig = schema("nutanixclusterconfigs")
	NutanixWorkerConfig  = schema("nutanixworkernodeconfigs")
)

func schema(resource string) clusterv1.VariableSchema {
	crd, err := crds.ReadFile("crds/caren.nutanix.com_" + resource + ".yaml")
	if err != nil {
		panic(err)
	}
	return variables.MustSchemaFromCRDYAML(crd)
}
