// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomainrollout

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
		"failure-domain-rollout-enabled",
		true,
		"Enable failure domain rollout controller to monitor cluster.status.failureDomains and trigger rollouts on "+
			"KubeAdmControlPlane when there are meaningful changes.",
	)

	pflag.CommandLine.IntVar(
		&o.Concurrency,
		"failure-domain-rollout-concurrency",
		10,
		"Number of Clusters to handle concurrently for failure domain rollout monitoring.",
	)
}
