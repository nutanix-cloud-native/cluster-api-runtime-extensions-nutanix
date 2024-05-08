// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func MustMarshalValuesToEnumJSON[T any](vals ...T) []apiextensionsv1.JSON {
	enumJSON := make([]apiextensionsv1.JSON, 0, len(vals))

	for _, v := range vals {
		enumJSON = append(enumJSON, *MustMarshal(v))
	}

	return enumJSON
}
