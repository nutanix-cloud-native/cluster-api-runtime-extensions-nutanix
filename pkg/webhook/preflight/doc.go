// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package preflight

// +kubebuilder:webhook:path=/preflight-v1beta1-cluster,mutating=false,failurePolicy=fail,groups="cluster.x-k8s.io",resources=clusters,verbs=create;update,versions=*,name=preflight.cluster.caren.nutanix.com,admissionReviewVersions=v1,sideEffects=None,timeoutSeconds=30

// NOTE The webhook is not configured to handle the status subresource. This means that update
// operations on the status subresource will not trigger the webhook.

// IMPORTANT Keep timeoutSeconds in sync with the `Timeout` constant defined in this package.
