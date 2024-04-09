// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package options

import (
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
)

func NewGlobalOptions() *GlobalOptions {
	return &GlobalOptions{}
}

type GlobalOptions struct {
	defaultsNamespace       string
	helmAddonsConfigMapName string
}

func (o *GlobalOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(
		&o.defaultsNamespace,
		"defaults-namespace",
		corev1.NamespaceDefault,
		"namespace for default configurations",
	)
	flags.StringVar(
		&o.helmAddonsConfigMapName,
		"helm-addons-configmap",
		"default-helm-addons-config",
		"Name of helm addons configmap",
	)
}

func (o *GlobalOptions) DefaultsNamespace() string {
	return o.defaultsNamespace
}

func (o *GlobalOptions) HelmAddonsConfigMapName() string {
	return o.helmAddonsConfigMapName
}
