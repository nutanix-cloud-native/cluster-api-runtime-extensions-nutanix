// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Get will get the value of the supplied annotation.
func Get(obj metav1.Object, name string) (value string, found bool) {
	annotations := obj.GetAnnotations()
	if len(annotations) == 0 {
		return "", false
	}

	value, found = annotations[name]

	return
}
