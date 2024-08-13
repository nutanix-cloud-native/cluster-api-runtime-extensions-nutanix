// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:webhook:path=/mutate-v1beta1-cluster,mutating=true,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create;update,versions=*,name=cluster-defaulter.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None
// +kubebuilder:webhook:path=/validate-v1beta1-cluster,mutating=false,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create;update,versions=*,name=cluster-validator.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None
package cluster
