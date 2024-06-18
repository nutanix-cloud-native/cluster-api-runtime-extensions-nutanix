// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Concurrency             int
	SourceNamespace         string
	TargetNamespaceLabelKey string
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	pflag.CommandLine.IntVar(
		&o.Concurrency,
		"namespacesync-concurrency",
		10,
		"Number of target namespaces to sync concurrently.",
	)

	pflag.CommandLine.StringVar(
		&o.SourceNamespace,
		"namespacesync-source-namespace",
		corev1.NamespaceDefault,
		"Namespace from which ClusterClasses and Templates are copied.",
	)

	pflag.CommandLine.StringVar(
		&o.TargetNamespaceLabelKey,
		"namespacesync-target-namespace-label-key",
		"",
		"Label key to determine if a namespace is a target. If a namespace has a label with this key, copy ClusterClasses and Templates to it from the source namespace.", //nolint:lll // Output will be wrapped.
	)
}
