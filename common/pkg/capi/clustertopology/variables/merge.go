// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"
	"fmt"
	"maps"

	"dario.cat/mergo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// MergeVariableOverridesWithGlobal merges the provided variable overrides with the global variables.
// It performs a deep merge, ensuring that if a variable exists in both maps, the value from the overrides is used.
func MergeVariableOverridesWithGlobal(
	overrideVars, globalVars map[string]apiextensionsv1.JSON,
) (map[string]apiextensionsv1.JSON, error) {
	mergedVars := maps.Clone(overrideVars)

	for k, v := range globalVars {
		// If the value of v is nil, skip it.
		if v.Raw == nil {
			continue
		}

		existingValue, exists := mergedVars[k]

		// If the variable does not exist in the mergedVars or the value is nil, add it and continue.
		if !exists || existingValue.Raw == nil {
			mergedVars[k] = v
			continue
		}

		// Wrap the value in a temporary key to ensure we can unmarshal to a map.
		// This is necessary because the values could be scalars.
		tempValJSON := fmt.Sprintf(`{"value": %s}`, string(existingValue.Raw))
		tempGlobalValJSON := fmt.Sprintf(`{"value": %s}`, string(v.Raw))

		// Unmarshal the existing value and the global value into maps.
		var val, globalVal map[string]interface{}
		if err := json.Unmarshal([]byte(tempValJSON), &val); err != nil {
			return nil, fmt.Errorf("failed to unmarshal existing value for key %q: %w", k, err)
		}

		if err := json.Unmarshal([]byte(tempGlobalValJSON), &globalVal); err != nil {
			return nil, fmt.Errorf("failed to unmarshal global value for key %q: %w", k, err)
		}

		// Now use mergo to perform a deep merge of the values, retaining the values in `val` if present.
		if err := mergo.Merge(&val, globalVal); err != nil {
			return nil, fmt.Errorf("failed to merge values for key %q: %w", k, err)
		}

		// Marshal the merged value back to JSON.
		mergedVal, err := json.Marshal(val["value"])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal merged value for key %q: %w", k, err)
		}

		mergedVars[k] = apiextensionsv1.JSON{Raw: mergedVal}
	}

	return mergedVars, nil
}
