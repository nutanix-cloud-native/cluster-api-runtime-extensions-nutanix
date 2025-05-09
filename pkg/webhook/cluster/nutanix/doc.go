// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:webhook:path=/mutate-v1beta1-cluster-nutanix,mutating=true,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create;update,versions=*,name=default.nutanix.cluster.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None
// +kubebuilder:webhook:path=/validate-v1beta1-cluster-nutanix,mutating=false,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create;update,versions=*,name=validate.nutanix.cluster.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None
package nutanix
