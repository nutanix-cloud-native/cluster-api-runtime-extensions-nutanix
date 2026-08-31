// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"context"
	"testing"
	"time"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
)

func Test_helmChartProxyIsReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  *caaphv1.HelmChartProxy
		want bool
	}{
		{
			name: "unobserved object is not ready",
			obj: &caaphv1.HelmChartProxy{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			},
		},
		{
			name: "zero generation and zero observed generation is not ready",
			obj:  &caaphv1.HelmChartProxy{},
		},
		{
			name: "stale observed generation is not ready",
			obj: &caaphv1.HelmChartProxy{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Status: caaphv1.HelmChartProxyStatus{
					ObservedGeneration: 1,
					Conditions: []metav1.Condition{{
						Type:   clusterv1.ReadyCondition,
						Status: metav1.ConditionTrue,
					}},
				},
			},
		},
		{
			name: "reconciled but not ready",
			obj: &caaphv1.HelmChartProxy{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: caaphv1.HelmChartProxyStatus{
					ObservedGeneration: 1,
					Conditions: []metav1.Condition{{
						Type:   clusterv1.ReadyCondition,
						Status: metav1.ConditionFalse,
						Reason: "IssuesReported",
					}},
				},
			},
		},
		{
			name: "reconciled and ready",
			obj: &caaphv1.HelmChartProxy{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: caaphv1.HelmChartProxyStatus{
					ObservedGeneration: 1,
					Conditions: []metav1.Condition{{
						Type:   clusterv1.ReadyCondition,
						Status: metav1.ConditionTrue,
					}},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			g.Expect(helmChartProxyIsReady(tt.obj)).To(gomega.Equal(tt.want))
		})
	}
}

func Test_remainingTooShortForWait(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	skip, remaining := remainingTooShortForWait(context.Background(), 30*time.Second)
	g.Expect(skip).To(gomega.BeFalse())
	g.Expect(remaining).To(gomega.Equal(time.Duration(0)))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	skip, remaining = remainingTooShortForWait(ctx, 30*time.Second)
	g.Expect(skip).To(gomega.BeTrue())
	g.Expect(remaining).To(gomega.BeNumerically(">", 0))
	g.Expect(remaining).To(gomega.BeNumerically("<=", 10*time.Second))

	longCtx, longCancel := context.WithTimeout(context.Background(), time.Minute)
	defer longCancel()
	skip, _ = remainingTooShortForWait(longCtx, 30*time.Second)
	g.Expect(skip).To(gomega.BeFalse())
}

func Test_conditionSummaries(t *testing.T) {
	t.Parallel()

	g := gomega.NewWithT(t)
	g.Expect(conditionSummaries(nil)).To(gomega.Equal("<none>"))
	g.Expect(conditionSummaries([]metav1.Condition{{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  "Failed",
		Message: "install timed out",
	}})).To(gomega.Equal(`Ready=False reason=Failed msg="install timed out"`))
}
