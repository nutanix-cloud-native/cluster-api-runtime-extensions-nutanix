// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
