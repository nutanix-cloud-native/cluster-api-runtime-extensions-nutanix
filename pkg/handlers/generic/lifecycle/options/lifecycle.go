package options

import "github.com/spf13/pflag"

type Options struct {
	HelmAddonsConfigMapName string
}

func New() *Options {
	return &Options{}
}

func (l *Options) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&l.HelmAddonsConfigMapName,
		prefix+"helm-addons-configmap",
		"default-helm-addons-config",
		"Name of helm addons configmap",
	)
}
