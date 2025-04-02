package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/util/yaml"
)

func UnstructuredFromBytes(objsYAML []byte, namespace string) ([]unstructured.Unstructured, error) {
	objs, err := yaml.ToUnstructured(objsYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	for i := range objs {
		objs[i].SetNamespace(namespace)
	}
	return objs, nil
}
