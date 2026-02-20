// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package skip

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestNew(t *testing.T) {
	testCases := []struct {
		name             string
		annotations      map[string]string
		expectCheckNames map[string]struct{}
	}{
		{
			name:             "ignores nil annotations",
			annotations:      nil,
			expectCheckNames: map[string]struct{}{},
		},
		{
			name:             "ignores empty annotations",
			annotations:      map[string]string{},
			expectCheckNames: map[string]struct{}{},
		},
		{
			name: "ignores missing annotation key",
			annotations: map[string]string{
				"some-other-key": "value",
			},
			expectCheckNames: map[string]struct{}{},
		},
		{
			name: "ignores empty skip annotation value",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "",
			},
			expectCheckNames: map[string]struct{}{},
		},
		{
			name: "ignores empty check names",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "InfraVMImage,,InfraCredentials,",
			},
			expectCheckNames: map[string]struct{}{
				"infravmimage":     {},
				"infracredentials": {},
			},
		},
		{
			name: "ignores value of only commas",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: ",,,",
			},
			expectCheckNames: map[string]struct{}{},
		},
		{
			name: "accepts single check name",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "InfraVMImage",
			},
			expectCheckNames: map[string]struct{}{
				"infravmimage": {},
			},
		},
		{
			name: "accepts multiple check names",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "InfraVMImage,InfraCredentials",
			},
			expectCheckNames: map[string]struct{}{
				"infravmimage":     {},
				"infracredentials": {},
			},
		},
		{
			name: "trims spaces from check names",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: " InfraVMImage , InfraCredentials ",
			},
			expectCheckNames: map[string]struct{}{
				"infravmimage":     {},
				"infracredentials": {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}

			evaluator := New(cluster)
			assert.Equal(t, tc.expectCheckNames, evaluator.normalizedCheckNames)
		})
	}
}

func TestEvaluator_For(t *testing.T) {
	testCases := []struct {
		name        string
		annotations map[string]string
		checkName   string
		expectMatch bool
	}{
		{
			name:        "no annotations",
			annotations: nil,
			checkName:   "InfraVMImage",
			expectMatch: false,
		},
		{
			name:        "no skip annotation",
			annotations: map[string]string{},
			checkName:   "InfraVMImage",
			expectMatch: false,
		},
		{
			name: "empty annotation value",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "",
			},
			checkName:   "InfraVMImage",
			expectMatch: false,
		},
		{
			name: "skip InfraVMImage check",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "InfraVMImage,InfraCredentials",
			},
			checkName:   "InfraVMImage",
			expectMatch: true,
		},
		{
			name: "skip credentials, but not image",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "InfraCredentials",
			},
			checkName:   "InfraVMImage",
			expectMatch: false,
		},
		{
			name: "skip with spaces and case mixing",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: " infraVMImage , InfraCredentials ",
			},
			checkName:   "InfraVMImage",
			expectMatch: true,
		},
		{
			name: "extra commas do not affect matching",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "InfraVMImage,,InfraCredentials,",
			},
			checkName:   "InfraVMImage",
			expectMatch: true,
		},
		{
			name: "skip all checks",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "all",
			},
			checkName:   "InfraVMImage",
			expectMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}

			evaluator := New(cluster)
			result := evaluator.For(tc.checkName)
			assert.Equal(t, tc.expectMatch, result)
		})
	}
}

func TestEvaluator_ForAll(t *testing.T) {
	testCases := []struct {
		name        string
		annotations map[string]string
		expectMatch bool
	}{
		{
			name:        "no annotations",
			annotations: nil,
			expectMatch: false,
		},
		{
			name:        "no skip annotation",
			annotations: map[string]string{},
			expectMatch: false,
		},
		{
			name: "empty annotation value",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "",
			},
		},
		{
			name: "skip all checks with spaces and case mixing",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: " aLL ",
			},
			expectMatch: true,
		},
		{
			name: "skip all checks with extra commas",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: ",all,,",
			},
			expectMatch: true,
		},
		{
			name: "skip all checks",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "all",
			},
			expectMatch: true,
		},
		{
			name: "skip some checks, but not all",
			annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: "OneCheck,AnotherCheck",
			},
			expectMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}

			evaluator := New(cluster)
			result := evaluator.ForAll()
			assert.Equal(t, tc.expectMatch, result)
		})
	}
}
