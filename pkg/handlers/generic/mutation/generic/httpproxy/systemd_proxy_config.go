// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

var (
	//go:embed templates/systemd.conf.tmpl
	systemdProxyTemplateContents string

	systemdProxyTemplate *template.Template = template.Must(
		template.New("").Parse(systemdProxyTemplateContents),
	)

	systemdUnitPaths = []string{
		"/etc/systemd/system/containerd.service.d/http-proxy.conf",
		"/etc/systemd/system/kubelet.service.d/http-proxy.conf",
	}
)

func generateSystemdFiles(vars v1alpha1.HTTPProxy, noProxy []string) []bootstrapv1.File {
	if vars.HTTP == "" && vars.HTTPS == "" && len(vars.AdditionalNo) == 0 {
		return nil
	}

	tplVars := struct {
		HTTP  string
		HTTPS string
		NO    string
	}{
		HTTP:  vars.HTTP,
		HTTPS: vars.HTTPS,
		NO:    strings.Join(noProxy, ","),
	}

	var buf bytes.Buffer
	err := systemdProxyTemplate.Execute(&buf, tplVars)
	if err != nil {
		panic(err) // This should be impossible with the simple template we have.
	}

	files := make([]bootstrapv1.File, 0, len(systemdUnitPaths))

	for _, path := range systemdUnitPaths {
		files = append(files, bootstrapv1.File{
			Path:        path,
			Permissions: "0640",
			Content:     buf.String(),
			Owner:       "root",
		})
	}
	return files
}
