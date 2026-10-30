// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type mockCheck struct {
	name   string
	result preflight.CheckResult
}

func (m *mockCheck) Name() string {
	return m.name
}

func (m *mockCheck) Run(ctx context.Context) preflight.CheckResult {
	return m.result
}

func TestNutanixChecker_Init(t *testing.T) {
	t.Parallel()

	var calls []string
	record := func(name string) []preflight.Check {
		calls = append(calls, name)
		return nil
	}

	checker := &nutanixChecker{
		configurationCheckFactory: func(cd *checkDependencies) preflight.Check {
			calls = append(calls, "configuration")
			return &mockCheck{name: "configuration"}
		},
		credentialsCheckFactory: func(
			ctx context.Context,
			nclientFactory func(prismgoclient.Credentials, types.NamespacedName, string) (client, error),
			cd *checkDependencies,
		) preflight.Check {
			calls = append(calls, "credentials")
			return &mockCheck{name: "credentials"}
		},
		prismCentralVersionCheckFactory: func(ctx context.Context, cd *checkDependencies) preflight.Check {
			calls = append(calls, "prismCentralVersion")
			return &mockCheck{name: "prismCentralVersion"}
		},
		failureDomainCheckFactory: func(cd *checkDependencies) []preflight.Check {
			return record("failureDomain")
		},
		vmImageChecksFactory: func(cd *checkDependencies) []preflight.Check {
			return record("vmImage")
		},
		vmImageKubernetesVersionChecksFactory: func(cd *checkDependencies) []preflight.Check {
			return record("vmImageKubernetesVersion")
		},
		cidrValidationChecksFactory: func(cd *checkDependencies) []preflight.Check {
			return record("cidrValidation")
		},
		storageContainerChecksFactory: func(cd *checkDependencies) []preflight.Check {
			return record("storageContainer")
		},
		controlPlaneEndpointChecksFactory: func(cd *checkDependencies) []preflight.Check {
			return record("controlPlaneEndpoint")
		},
		metroChecksFactory: func(cd *checkDependencies) []preflight.Check {
			return record("metro")
		},
		flowNetworkSecurityDSRToSNATCheckerFactory: func(cd *checkDependencies) []preflight.Check {
			return record("flowNetworkSecurityDSRToSNAT")
		},
	}

	checker.Init(context.Background(), nil, &clusterv1beta2.Cluster{}, nil)

	if !assert.Len(t, calls, 11) {
		return
	}
	assert.Equal(t, []string{"configuration", "credentials", "prismCentralVersion"}, calls[:3])
	assert.ElementsMatch(t, []string{
		"failureDomain",
		"vmImage",
		"vmImageKubernetesVersion",
		"cidrValidation",
		"storageContainer",
		"controlPlaneEndpoint",
		"metro",
		"flowNetworkSecurityDSRToSNAT",
	}, calls[3:])
}

func TestNutanixChecker_PrismCentralVersionGating(t *testing.T) {
	t.Parallel()

	type scenario struct {
		name                    string
		annotations             map[string]string
		versionCheckResult      preflight.CheckResult
		expectedDependentChecks int
	}

	scenarios := []scenario{
		{
			name: "skip version check lets dependent run",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "NutanixPrismCentralVersion",
			},
			versionCheckResult:      preflight.CheckResult{Allowed: true},
			expectedDependentChecks: 1,
		},
		{
			name: "version failure skips dependent",
			versionCheckResult: preflight.CheckResult{
				Allowed: false,
			},
			expectedDependentChecks: 0,
		},
		{
			name: "version success runs dependent",
			versionCheckResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedDependentChecks: 1,
		},
	}

	for _, tt := range scenarios {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			versionRunCount := 0

			checker := &nutanixChecker{
				configurationCheckFactory: func(cd *checkDependencies) preflight.Check {
					return &mockCheck{
						name:   "config",
						result: preflight.CheckResult{Allowed: true},
					}
				},
				credentialsCheckFactory: func(
					ctx context.Context,
					nclientFactory func(prismgoclient.Credentials, types.NamespacedName, string) (client, error),
					cd *checkDependencies,
				) preflight.Check {
					cd.nclient = &clientWrapper{}
					return &mockCheck{
						name:   "creds",
						result: preflight.CheckResult{Allowed: true},
					}
				},
				prismCentralVersionCheckFactory: func(ctx context.Context, cd *checkDependencies) preflight.Check {
					// If already validated or skipped (set from annotation), skip API call
					if cd != nil && cd.pcVersion != "" {
						return &mockCheck{
							name:   "NutanixPrismCentralVersion",
							result: preflight.CheckResult{Allowed: true},
						}
					}
					versionRunCount++
					if cd != nil {
						if tt.versionCheckResult.Allowed {
							cd.pcVersion = "7.3.0"
						} else {
							cd.pcVersion = ""
						}
					}
					return &mockCheck{
						name:   "NutanixPrismCentralVersion",
						result: tt.versionCheckResult,
					}
				},
				failureDomainCheckFactory: func(cd *checkDependencies) []preflight.Check {
					if cd == nil || cd.pcVersion == "" {
						return nil
					}
					return []preflight.Check{
						&mockCheck{
							name:   "DependentCheck",
							result: preflight.CheckResult{Allowed: true},
						},
					}
				},
				vmImageChecksFactory: func(cd *checkDependencies) []preflight.Check {
					return nil
				},
				vmImageKubernetesVersionChecksFactory: func(cd *checkDependencies) []preflight.Check {
					return nil
				},
				cidrValidationChecksFactory: func(cd *checkDependencies) []preflight.Check {
					return nil
				},
				storageContainerChecksFactory: func(cd *checkDependencies) []preflight.Check {
					return nil
				},
				controlPlaneEndpointChecksFactory: func(cd *checkDependencies) []preflight.Check {
					return nil
				},
				metroChecksFactory: func(cd *checkDependencies) []preflight.Check {
					return nil
				},
				flowNetworkSecurityDSRToSNATCheckerFactory: func(cd *checkDependencies) []preflight.Check {
					return nil
				},
			}

			cluster := &clusterv1beta2.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}

			checks := checker.Init(ctx, nil, cluster, nil)

			dependentCount := 0
			hasVersionCheck := false
			for _, check := range checks {
				if check.Name() == "NutanixPrismCentralVersion" {
					hasVersionCheck = true
				}
				if check.Name() == "DependentCheck" {
					dependentCount++
				}
			}

			assert.True(t, hasVersionCheck)
			assert.Equal(t, tt.expectedDependentChecks, dependentCount)
		})
	}
}
