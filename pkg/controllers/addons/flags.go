// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"github.com/spf13/pflag"
)

type Options struct {
	Concurrency int
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	pflag.CommandLine.IntVar(
		&o.Concurrency,
		"addons-clusters-concurrency",
		10,
		"Number of clusters to sync concurrently.",
	)
}
