// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	yamlDecode "k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
)

type HelmChartFromConfigMap struct {
	Name       string   `yaml:"ChartName"`
	Version    []string `yaml:"ChartVersions"`
	Repository string   `yaml:"RepositoryURL"`
}

type Repository struct {
	RepoURL string              `yaml:"repoURL,omitempty"`
	Charts  map[string][]string `yaml:"charts,omitempty"`
}

type HelmChartsConfig struct {
	Repositories map[string]Repository `yaml:"repositories,omitempty"`
}

var log = ctrl.LoggerFrom(context.Background())

func main() {
	args := os.Args
	var (
		outputFile         string
		inputConfigMapFile string
	)
	flagSet := flag.NewFlagSet("mindthegap-helm-registry", flag.ExitOnError)
	flagSet.StringVar(&outputFile, "output-file", "",
		"output file name to write config map to.")
	flagSet.StringVar(&inputConfigMapFile, "input-configmap-file", "",
		"input configmap file to create the mindthegap repo file from")
	err := flagSet.Parse(args[1:])
	if err != nil {
		log.Error(err, "failed to parse args")
	}
	fullPath := inputConfigMapFile
	if !path.IsAbs(fullPath) {
		wd, err := os.Getwd()
		if err != nil {
			log.Error(err, "failed to get wd")
			return
		}
		fullPath = path.Join(wd, inputConfigMapFile)
	}
	f, err := os.Open(fullPath)
	if err != nil {
		log.Error(err, "failed to open file")
		return
	}
	defer f.Close()
	cm := &corev1.ConfigMap{}
	err = yamlDecode.NewYAMLOrJSONDecoder(f, 1024).Decode(cm)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to unmarshal file %s", fullPath))
	}
	out := HelmChartsConfig{
		map[string]Repository{},
	}
	for _, info := range cm.Data {
		var settings HelmChartFromConfigMap
		err = yaml.Unmarshal([]byte(info), &settings)
		if err != nil {
			log.Error(err, "failed unmarshl settings")
			return
		}
		out.Repositories[settings.Name] = Repository{
			RepoURL: settings.Repository,
			Charts: map[string][]string{
				settings.Name: {
					settings.Version,
				},
			},
		}
	}
	b, err := yaml.Marshal(out)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to marshal obj %v", out))
	}
	fullOutputfilePath := outputFile
	if !path.IsAbs(outputFile) {
		wd, err := os.Getwd()
		if err != nil {
			log.Error(err, "failed")
		}
		fullOutputfilePath = path.Join(wd, outputFile)
	}
	f, err = os.OpenFile(fullOutputfilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		log.Error(err, "failed to create file")
	}
	defer f.Close()
	_, err = bytes.NewBuffer(b).WriteTo(f)
	if err != nil {
		log.Error(err, "failed to write to file")
	}
}
