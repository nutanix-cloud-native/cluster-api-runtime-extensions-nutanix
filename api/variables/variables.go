// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"
	"fmt"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
)

func UnmarshalRuntimeVariable[T any](runtimeVariable *runtimehooksv1.Variable, obj *T) error {
	err := json.Unmarshal(runtimeVariable.Value.Raw, obj)
	if err != nil {
		return fmt.Errorf("error unmarshalling variable: %w", err)
	}

	return nil
}

//nolint:gocritic // no need for named results
func GetRuntimhookVariableByName(
	name string,
	variables []runtimehooksv1.Variable,
) (*runtimehooksv1.Variable, int) {
	for i, runtimevar := range variables {
		if runtimevar.Name == name {
			return &runtimevar, i
		}
	}
	return nil, -1
}
