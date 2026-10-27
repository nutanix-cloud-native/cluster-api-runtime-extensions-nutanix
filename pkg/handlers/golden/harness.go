// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package golden runs every registered topology mutation handler against a frozen corpus of Clusters and compares the
// result with committed golden files, so that any change to a released handler's output fails the build.
package golden

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// TemplateKey identifies a template referenced by the ClusterClass.
type TemplateKey struct {
	Kind string
	Name string
}

// Input is one corpus file: a ClusterClass with its templates, a Cluster using it, and any objects the handlers read
// from the management cluster.
type Input struct {
	ClusterClass *clusterv1.ClusterClass
	Cluster      *clusterv1.Cluster
	Templates    map[TemplateKey]*unstructured.Unstructured
	Fixtures     []*unstructured.Unstructured
}

func LoadInput(path string) (*Input, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	in := &Input{Templates: map[TemplateKey]*unstructured.Unstructured{}}
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		u := &unstructured.Unstructured{}
		if err := decoder.Decode(&u.Object); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding %s: %w", path, err)
		}
		if len(u.Object) == 0 {
			continue
		}

		switch kind := u.GetKind(); {
		case kind == "ClusterClass":
			in.ClusterClass = &clusterv1.ClusterClass{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, in.ClusterClass)
		case kind == "Cluster":
			in.Cluster = &clusterv1.Cluster{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, in.Cluster)
		case strings.HasSuffix(kind, "Template"):
			in.Templates[TemplateKey{Kind: kind, Name: u.GetName()}] = u
		default:
			in.Fixtures = append(in.Fixtures, u)
		}
		if err != nil {
			return nil, fmt.Errorf("converting %s %s in %s: %w", u.GetKind(), u.GetName(), path, err)
		}
	}

	if in.ClusterClass == nil || in.Cluster == nil {
		return nil, fmt.Errorf("%s must contain one ClusterClass and one Cluster", path)
	}
	return in, nil
}
