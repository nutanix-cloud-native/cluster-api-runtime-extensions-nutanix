// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"path/filepath"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func main() {
	outputDir := flag.String("output-dir", "", "output directory")
	flag.Parse()

	type hasSchema interface {
		VariableSchema() clusterv1.VariableSchema
	}

	for _, obj := range []struct {
		typ     hasSchema
		typName string
	}{
		{v1alpha1.NewAWSClusterConfigSpec(), "awsclusterconfig"},
		{v1alpha1.NewAWSWorkerConfigSpec(), "awsworkerconfig"},
		{v1alpha1.ClusterConfigSpec{Docker: &v1alpha1.DockerSpec{}}, "dockerclusterconfig"},
		{v1alpha1.NodeConfigSpec{Docker: &v1alpha1.DockerNodeSpec{}}, "dockerworkerconfig"},
		{v1alpha1.ClusterConfigSpec{Nutanix: &v1alpha1.NutanixSpec{}}, "nutanixclusterconfig"},
		{v1alpha1.NodeConfigSpec{Nutanix: &v1alpha1.NutanixNodeSpec{}}, "nutanixworkerconfig"},
		{v1alpha1.ClusterConfigSpec{}, "genericclusterconfig"},
	} {
		exportedYAML, err := yaml.Marshal(obj.typ.VariableSchema())
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(*outputDir, obj.typName+".yaml"), exportedYAML, 0o600); err != nil {
			panic(err)
		}
	}
}
