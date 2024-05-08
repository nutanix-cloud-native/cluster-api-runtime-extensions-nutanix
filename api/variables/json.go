// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func MustMarshal(val any) *apiextensionsv1.JSON {
	marshaled, err := json.Marshal(val)
	if err != nil {
		panic(fmt.Errorf("failed to marshal enum value: %w", err))
	}

	return &apiextensionsv1.JSON{Raw: marshaled}
}
