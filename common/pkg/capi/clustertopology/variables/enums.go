// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func ValuesToEnumJSON[T any](vals ...T) ([]apiextensionsv1.JSON, error) {
	enumJSON := make([]apiextensionsv1.JSON, 0, len(vals))

	for _, v := range vals {
		enumVal, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal enum value: %v", v)
		}
		enumJSON = append(enumJSON, apiextensionsv1.JSON{Raw: enumVal})
	}

	return enumJSON, nil
}
