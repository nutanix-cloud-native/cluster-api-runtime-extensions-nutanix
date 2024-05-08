// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Get will get the value of the supplied annotation.
func Get(obj metav1.Object, name string) (string, bool) {
	annotations := obj.GetAnnotations()
	val, found := annotations[name]
	return val, found
}
