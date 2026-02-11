// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package conditions

import (
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Getter interface defines methods that a Cluster API object should implement in order to
// use the conditions package for getting conditions.
//
// This was copied from the CAPI annotations package to support deprecated v1beta1.
type Getter interface {
	client.Object

	// GetConditions returns the list of conditions for a cluster API object.
	GetConditions() clusterv1.Conditions
}

// Get returns the condition with the given type, if the condition does not exist,
// it returns nil.
//
// This was copied from the CAPI annotations package to support deprecated v1beta1.
func Get(from Getter, t clusterv1.ConditionType) *clusterv1.Condition {
	conditions := from.GetConditions()
	if conditions == nil {
		return nil
	}

	for _, condition := range conditions {
		if condition.Type == t {
			return &condition
		}
	}
	return nil
}

// IsTrue is true if the condition with the given type is True, otherwise it returns false
// if the condition is not True or if the condition does not exist (is nil).
//
// This was copied from the CAPI annotations package to support deprecated v1beta1.
func IsTrue(from Getter, t clusterv1.ConditionType) bool {
	if c := Get(from, t); c != nil {
		return c.Status == corev1.ConditionTrue
	}
	return false
}
