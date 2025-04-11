// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	yamlDecode "k8s.io/apimachinery/pkg/util/yaml"
)

type HelmChartFromConfigMap struct {
	Name       string `yaml:"ChartName"`
	Version    string `yaml:"ChartVersion"`
	Repository string `yaml:"RepositoryURL"`
}

type Repository struct {
	RepoURL string              `yaml:"repoURL,omitempty"`
	Charts  map[string][]string `yaml:"charts,omitempty"`
}

type HelmChartsConfig struct {
	Repositories map[string]Repository `yaml:"repositories,omitempty"`
	ChartURLs    []string              `yaml:"chartURLs,omitempty"`
}

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
		log.Fatalln("failed to parse args:", err)
	}
	if outputFile == "" {
		log.Fatalln("output file is required")
	}
	if inputConfigMapFile == "" {
		log.Fatalln("input configmap file is required")
	}
	fullPath := inputConfigMapFile
	if !path.IsAbs(fullPath) {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalln("failed to get wd:", err)
		}
		fullPath = path.Join(wd, inputConfigMapFile)
	}
	f, err := os.Open(fullPath)
	if err != nil {
		log.Fatalln("failed to open file:", err)
	}
	defer f.Close()
	cm := &corev1.ConfigMap{}
	err = yamlDecode.NewYAMLOrJSONDecoder(f, 1024).Decode(cm)
	if err != nil {
		f.Close()
		log.Fatalf("failed to unmarshal file %s: %v\n", fullPath, err)
	}
	out := HelmChartsConfig{
		map[string]Repository{},
		[]string{},
	}
	for _, info := range cm.Data {
		var settings HelmChartFromConfigMap
		err = yaml.Unmarshal([]byte(info), &settings)
		if err != nil {
			log.Fatalln("failed to unmarshal settings:", err)
		}
		if strings.HasPrefix(settings.Repository, "oci://") {
			url := fmt.Sprintf("%s/%s:%s", settings.Repository, settings.Name, settings.Version)
			out.ChartURLs = append(out.ChartURLs, url)
		} else {
			out.Repositories[settings.Name] = Repository{
				RepoURL: settings.Repository,
				Charts: map[string][]string{
					settings.Name: {
						settings.Version,
					},
				},
			}
		}
	}
	b, err := yaml.Marshal(out)
	if err != nil {
		log.Fatalf("failed to marshal obj %+v: %v", out, err)
	}
	fullOutputfilePath := outputFile
	if !path.IsAbs(outputFile) {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalln("failed:", err)
		}
		fullOutputfilePath = path.Join(wd, outputFile)
	}
	f, err = os.OpenFile(fullOutputfilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		log.Fatalln("failed to create file:", err)
	}
	defer f.Close()
	_, err = bytes.NewBuffer(b).WriteTo(f)
	if err != nil {
		log.Fatalln("failed to write to file:", err)
	}
}
