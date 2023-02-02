// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/repository"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/yamlprocessor"
)

func crsObjsFromTemplates(ns string, templates ...[]byte) ([]unstructured.Unstructured, error) {
	objs := make([]unstructured.Unstructured, 0)
	for _, t := range templates {
		o, err := objsFromTemplate(t, ns)
		if err != nil {
			return nil, err
		}
		objs = append(objs, o...)
	}
	return objs, nil
}

func objsFromTemplate(template []byte, ns string) ([]unstructured.Unstructured, error) {
	ti := repository.TemplateInput{
		RawArtifact:     template,
		TargetNamespace: ns,
		Processor:       yamlprocessor.NewSimpleProcessor(),
	}

	t, err := repository.NewTemplate(ti)
	if err != nil {
		return nil, fmt.Errorf("failed to generate template: %w", err)
	}

	return t.Objs(), nil
}
