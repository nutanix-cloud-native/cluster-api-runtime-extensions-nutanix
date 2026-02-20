// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package skip

import (
	"strings"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// Evaluator is used to determine which checks should be skipped, based on the cluster's annotations.
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

	value, exists := annotations[carenv1.PreflightChecksSkipAnnotationKey]
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
	if _, exists := o.normalizedCheckNames[carenv1.PreflightChecksSkipAllAnnotationValue]; exists &&
		len(o.normalizedCheckNames) == 1 {
		o.all = true
	}
	return o
}

// For checks if the specific check should be skipped.
// It returns true if the cluster skip annotation contains the check name.
// The check name is case-insensitive, so "CheckName1" and "checkname1" will both match.
// If the cluster has skipped all checks, For will return true for any check name.
//
// For example, if the cluster has skipped "CheckName1", then calling
// For("CheckName1") or For("checkname1") will return true, but For("CheckName2") will
// return false.
func (o *Evaluator) For(checkName string) bool {
	if o.all {
		// If the cluster has skipped all checks, return true for any check name.
		return true
	}
	normalizedCheckName := strings.TrimSpace(strings.ToLower(checkName))
	_, exists := o.normalizedCheckNames[normalizedCheckName]
	return exists
}

// ForAll checks if all checks should be skipped.
// It returns true if the cluster skip annotation contains "all".
// The check is case-insensitive, so "all", "ALL", and "All" will all match.
func (o *Evaluator) ForAll() bool {
	return o.all
}
