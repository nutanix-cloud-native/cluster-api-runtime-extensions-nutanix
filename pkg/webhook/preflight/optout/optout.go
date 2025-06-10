// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package optout

import (
	"strings"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// AnnotationKey is the key of the annotation on the Cluster used to opt out preflight checks.
	AnnotationKey = "preflight.cluster.caren.nutanix.com/opt-out"

	// OptOutAllChecksAnnotationValue is the value used in the cluster's annotations to indicate
	// that all checks are opted out.
	OptOutAllChecksAnnotationValue = "all"
)

type Evaluator struct {
	normalizedCheckNames map[string]struct{}
	all                  bool
}

// New creates a new Evaluator from the cluster's annotations.
func New(cluster *clusterv1.Cluster) *Evaluator {
	o := &Evaluator{
		normalizedCheckNames: make(map[string]struct{}),
	}

	annotations := cluster.GetAnnotations()
	if annotations == nil {
		// If there are no annotations, return an Evaluator with no prefixes.
		return o
	}

	value, exists := annotations[AnnotationKey]
	if !exists {
		// If the annotation does not exist, return an Evaluator with no prefixes.
		return o
	}

	for _, checkName := range strings.Split(value, ",") {
		if checkName == "" {
			// Ignore whitespace between commas.
			continue
		}
		normalizedCheckName := strings.TrimSpace(strings.ToLower(checkName))
		o.normalizedCheckNames[normalizedCheckName] = struct{}{}
	}
	if _, exists := o.normalizedCheckNames[OptOutAllChecksAnnotationValue]; exists && len(o.normalizedCheckNames) == 1 {
		o.all = true
	}
	return o
}

// For checks if the cluster has opted out of a specific check.
// It returns true if the cluster has opted out of the check with the given name.
// The check name is case-insensitive, so "CheckName1" and "checkname1" will both match.
// If the cluster has opted out of all checks, For will return true for any check name.
//
// For example, if the cluster has opted out of "CheckName1", then calling
// For("CheckName1") or For("checkname1") will return true, but For("CheckName2") will
// return false.
func (o *Evaluator) For(checkName string) bool {
	if o.all {
		// If the cluster has opted out of all checks, return true for any check name.
		return true
	}
	normalizedCheckName := strings.TrimSpace(strings.ToLower(checkName))
	_, exists := o.normalizedCheckNames[normalizedCheckName]
	return exists
}

// ForAll checks if the cluster has opted out of all checks.
// It returns true if the cluster has a single prefix "all" in its opt-out annotations.
// The check is case-insensitive, so "all", "ALL", and "All" will all match.
func (o *Evaluator) ForAll() bool {
	return o.all
}
