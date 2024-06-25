// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package namespacesync

import corev1 "k8s.io/api/core/v1"

var NamespaceHasLabelKey = func(key string) func(ns *corev1.Namespace) bool {
	return func(ns *corev1.Namespace) bool {
		_, ok := ns.GetLabels()[key]
		return ok
	}
}
