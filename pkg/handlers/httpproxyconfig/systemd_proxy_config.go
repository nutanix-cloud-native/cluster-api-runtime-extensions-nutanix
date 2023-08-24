// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxyconfig

import (
	"bytes"
	"embed"
	"strings"
	"text/template"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

//go:embed templates
var sources embed.FS

var templates *template.Template

func init() {
	templates = template.Must(template.ParseFS(sources, "templates/*.tmpl"))
}

var systemdUnitPaths = []string{
	"/etc/systemd/system/containerd.service.d/http-proxy.conf",
	"/etc/systemd/system/kubelet.service.d/http-proxy.conf",
}

type systemdConfigGenerator struct {
	template *template.Template
}

func (g *systemdConfigGenerator) Generate(vars HTTPProxyVariables) (string, error) {
	tplVars := struct {
		HTTP  string
		HTTPS string
		NO    string
	}{
		HTTP:  vars.HTTP,
		HTTPS: vars.HTTPS,
		NO:    strings.Join(vars.NO, ","),
	}

	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "systemd.conf.tmpl", tplVars)
	return buf.String(), err
}

func (g *systemdConfigGenerator) AddSystemdFiles(
	vars HTTPProxyVariables, dest []bootstrapv1.File,
) ([]bootstrapv1.File, error) {
	content, err := g.Generate(vars)
	if err != nil {
		return nil, err
	}

	for _, path := range systemdUnitPaths {
		dest = append(dest, bootstrapv1.File{
			Path:        path,
			Permissions: "0640",
			Content:     content,
		})
	}
	return dest, nil
}
