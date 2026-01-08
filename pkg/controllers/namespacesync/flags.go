// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"github.com/spf13/pflag"
)

type Options struct {
	Enabled                      bool
	Concurrency                  int
	SourceNamespace              string
	TargetNamespaceLabelSelector string
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(
		&o.Enabled,
		"namespacesync-enabled",
		false,
		"Enable copying of ClusterClasses and Templates from a source namespace to one or more target namespaces.",
	)

	flags.IntVar(
		&o.Concurrency,
		"namespacesync-concurrency",
		10,
		"Number of target namespaces to sync concurrently.",
	)

	flags.StringVar(
		&o.SourceNamespace,
		"namespacesync-source-namespace",
		"",
		"Namespace from which ClusterClasses and Templates are copied.",
	)

	flags.StringVar(
		&o.TargetNamespaceLabelSelector,
		"namespacesync-target-namespace-label-key",
		"",
		"Label key to determine if a namespace is a target. If a namespace has a label with this key, copy ClusterClasses and Templates to it from the source namespace.", //nolint:lll // Output will be wrapped.
	)
	_ = flags.MarkDeprecated(
		"namespacesync-target-namespace-label-key",
		"use namespacesync-target-namespace-label-selector instead",
	)
	flags.StringVar(
		&o.TargetNamespaceLabelSelector,
		"namespacesync-target-namespace-label-selector",
		"",
		"Label selector to determine target namespaces. Namespaces matching this selector will receive copies of ClusterClasses and Templates from the source namespace. Example: 'environment=production' or 'team in (platform,infrastructure)'.", //nolint:lll // Output will be wrapped.
	)
}
