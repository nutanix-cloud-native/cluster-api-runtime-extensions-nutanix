// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/lifecycle"
)

type addonProviderValue lifecycle.AddonProvider

func (v addonProviderValue) String() string {
	return string(v)
}

func (v *addonProviderValue) Set(value string) error {
	switch lifecycle.AddonProvider(value) {
	case lifecycle.ClusterResourceSetAddonProvider, lifecycle.FluxHelmReleaseAddonProvider:
		break
	default:
		return fmt.Errorf(
			"invalid addon provider: %q (must be one of %v)",
			value,
			[]string{
				string(lifecycle.ClusterResourceSetAddonProvider),
				string(lifecycle.FluxHelmReleaseAddonProvider),
			},
		)
	}

	*v = addonProviderValue(value)

	return nil
}

func (*addonProviderValue) Type() string {
	return "addonProvider"
}

func newAddonProviderValue(val lifecycle.AddonProvider, p *lifecycle.AddonProvider) pflag.Value {
	*p = val
	return (*addonProviderValue)(p)
}
