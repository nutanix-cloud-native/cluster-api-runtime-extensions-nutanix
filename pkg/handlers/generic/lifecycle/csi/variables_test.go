// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation_AWSCSI(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid aws-ebs CSI specified for AWS infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderAWSEBS,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderAWSEBS: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid nutanix CSI specified for AWS infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderNutanix,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderNutanix: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "invalid local-path CSI specified for AWS infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderLocalPath,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderLocalPath: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "invalid unknown CSI specified for AWS infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           "unknown",
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							"unknown": {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}

func TestVariableValidation_NutanixCSI(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid nutanix CSI specified for Nutanix infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderNutanix,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderNutanix: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid aws-ebs CSI specified for Nutanix infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderAWSEBS,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderAWSEBS: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "invalid local-path CSI specified for Nutanix infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderLocalPath,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderLocalPath: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "invalid unknown CSI specified for Nutanix infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           "unknown",
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							"unknown": {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}

func TestVariableValidation_DockerCSI(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid local-path CSI specified for Docker infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderLocalPath,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderLocalPath: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid aws-ebs CSI specified for Docker infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderAWSEBS,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderAWSEBS: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "invalid nutanix CSI specified for Docker infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           v1alpha1.CSIProviderNutanix,
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							v1alpha1.CSIProviderNutanix: {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "invalid unknown CSI specified for Docker infra provider",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					CSI: &apivariables.CSI{
						GenericCSI: v1alpha1.GenericCSI{
							DefaultStorage: v1alpha1.DefaultStorage{
								Provider:           "unknown",
								StorageClassConfig: "test-1",
							},
						},
						Providers: map[string]v1alpha1.CSIProvider{
							"unknown": {
								Strategy: ptr.To(v1alpha1.AddonStrategyHelmAddon),
								StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
									"test-1": {},
								},
							},
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}
