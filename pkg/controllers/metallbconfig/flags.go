// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallbconfig

import (
	"github.com/spf13/pflag"
)

type Options struct {
	Concurrency int
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.IntVar(
		&o.Concurrency,
		"metallb-config-concurrency",
		10,
		"Number of Clusters to handle concurrently for MetalLB configuration reconciliation.",
	)
}
