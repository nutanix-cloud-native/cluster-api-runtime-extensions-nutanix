// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package enforceclusterautoscalerlimits

import (
	"github.com/spf13/pflag"
)

type Options struct {
	Enabled     bool
	Concurrency int
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	pflag.CommandLine.BoolVar(
		&o.Enabled,
		"enforce-clusterautoscaler-limits-enabled",
		false,
		"Enable enforcing cluster-autoscaler limits on MachineDeployments.",
	)

	pflag.CommandLine.IntVar(
		&o.Concurrency,
		"enforce-clusterautoscaler-limits-concurrency",
		10,
		"Number of MachineDeployments to handle concurrently.",
	)
}
