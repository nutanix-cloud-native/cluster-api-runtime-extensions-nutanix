package envtest

import (
	"bytes"
	"os"
	"path/filepath"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// loadCRDsWithV1Beta2Storage reads CRD YAML files from the given directories
// and patches multi-version CRDs to use v1beta2 as the storage version.
//
// CAPI v1.12+ CRDs already default to v1beta2 as storage. This function
// explicitly sets v1beta2 as storage so that envtest (which lacks conversion
// webhooks) can properly round-trip v1beta2 objects. Without a conversion
// webhook, fields with renamed JSON tags (e.g. v1beta2 templateRef vs
// v1beta1 ref) would be lost when stored under a different version's schema.
func loadCRDsWithV1Beta2Storage(dirs []string) []*apiextensionsv1.CustomResourceDefinition {
	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			panic("failed to read CRD directory " + dir + ": " + err.Error())
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := filepath.Ext(entry.Name())
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				panic("failed to read CRD file " + entry.Name() + ": " + err.Error())
			}
			crd := &apiextensionsv1.CustomResourceDefinition{}
			if err := yaml.NewYAMLOrJSONDecoder(
				bytes.NewReader(data), 4096,
			).Decode(crd); err != nil {
				panic("failed to decode CRD from " + entry.Name() + ": " + err.Error())
			}
			forceV1Beta2Storage(crd)
			crds = append(crds, crd)
		}
	}
	return crds
}

// forceV1Beta2Storage patches a CRD so that v1beta2 is the storage version
// and all other versions are non-storage. CRDs without a v1beta2 version
// are left unmodified (they keep their existing storage version).
func forceV1Beta2Storage(crd *apiextensionsv1.CustomResourceDefinition) {
	hasV1Beta2 := false
	for _, v := range crd.Spec.Versions {
		if v.Name == "v1beta2" {
			hasV1Beta2 = true
			break
		}
	}
	if !hasV1Beta2 {
		return
	}
	for i := range crd.Spec.Versions {
		crd.Spec.Versions[i].Storage = crd.Spec.Versions[i].Name == "v1beta2"
	}
}
