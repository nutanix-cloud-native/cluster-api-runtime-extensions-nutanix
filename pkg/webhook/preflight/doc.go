// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

// +kubebuilder:webhook:path=/preflight-v1beta1,mutating=false,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create;update,versions=*,name=preflight.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None,timeoutSeconds=30
