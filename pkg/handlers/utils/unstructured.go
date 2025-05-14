package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/util/yaml"
)

// UnstructuredFromBytes converts a byte array containing YAML objects into a slice of unstructured.Unstructured.
// If the objects do not have a namespace set, it sets the namespace to the provided namespace.
func UnstructuredFromBytes(objsYAML []byte, namespace string) ([]unstructured.Unstructured, error) {
	objs, err := yaml.ToUnstructured(objsYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	for i := range objs {
		if objs[i].GetNamespace() == "" {
			objs[i].SetNamespace(namespace)
		}
	}
	return objs, nil
}
