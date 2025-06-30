// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:webhook:path=/mutate-v1beta1-addons,mutating=true,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create,versions=*,name=addons-defaulter.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None
package addons
