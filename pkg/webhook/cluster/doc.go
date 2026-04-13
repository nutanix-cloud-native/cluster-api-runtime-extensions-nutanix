// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:webhook:path=/mutate-cluster,mutating=true,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create;update,versions=v1beta2,name=cluster-defaulter.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None
// +kubebuilder:webhook:path=/validate-cluster,mutating=false,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create;update,versions=v1beta2,name=cluster-validator.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None
package cluster
