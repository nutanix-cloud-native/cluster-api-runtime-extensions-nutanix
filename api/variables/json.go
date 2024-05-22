// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func MarshalToClusterVariable[T any](name string, obj T) (*clusterv1.ClusterVariable, error) {
	marshaled, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal variable value %q: %w", name, err)
	}
	return &clusterv1.ClusterVariable{
		Name:  name,
		Value: apiextensionsv1.JSON{Raw: marshaled},
	}, nil
}

func UnmarshalClusterConfigVariable(
	clusterVariables []clusterv1.ClusterVariable,
) (*ClusterConfigSpec, error) {
	variableName := v1alpha1.ClusterConfigVariableName
	clusterConfig := GetClusterVariableByName(variableName, clusterVariables)
	if clusterConfig == nil {
		return nil, nil
	}
	spec := &ClusterConfigSpec{}
	err := UnmarshalClusterVariable(clusterConfig, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster variable %q: %w", variableName, err)
	}

	return spec, nil
}

func UnmarshalWorkerConfigVariable(
	clusterVariables []clusterv1.ClusterVariable,
) (*WorkerNodeConfigSpec, error) {
	variableName := v1alpha1.WorkerConfigVariableName
	workerConfig := GetClusterVariableByName(variableName, clusterVariables)
	if workerConfig == nil {
		return nil, nil
	}
	spec := &WorkerNodeConfigSpec{}
	err := UnmarshalClusterVariable(workerConfig, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster variable %q: %w", variableName, err)
	}

	return spec, nil
}

func UnmarshalClusterVariable[T any](clusterVariable *clusterv1.ClusterVariable, obj *T) error {
	err := json.Unmarshal(clusterVariable.Value.Raw, obj)
	if err != nil {
		return fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return nil
}

func GetClusterVariableByName(
	name string,
	clusterVariables []clusterv1.ClusterVariable,
) *clusterv1.ClusterVariable {
	for _, clusterVar := range clusterVariables {
		if clusterVar.Name == name {
			return &clusterVar
		}
	}
	return nil
}
