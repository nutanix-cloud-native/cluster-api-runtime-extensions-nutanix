// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"
	"fmt"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func MarshalToClusterVariable[T any](name string, obj T) (*clusterv1.ClusterVariable, error) {
	marshaled, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal variable value %q: %w", name, err)
	}
	return &clusterv1.ClusterVariable{
		Name:  name,
		Value: v1.JSON{Raw: marshaled},
	}, nil
}

func UnmarshalClusterVariable[T any](clusterVariable *clusterv1.ClusterVariable, obj *T) error {
	err := json.Unmarshal(clusterVariable.Value.Raw, obj)
	if err != nil {
		return fmt.Errorf("error unmarshalling variable: %w", err)
	}

	return nil
}
