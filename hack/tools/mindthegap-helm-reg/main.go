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
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	yamlDecode "k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
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
}

var log = ctrl.LoggerFrom(context.Background())

func main() {
	args := os.Args
	var (
		outputFile            string
		inputConfigMapFile    string
		previousConfigMapFile string
		nminus2ConfigMapFile  string
	)
	flagSet := flag.NewFlagSet("mindthegap-helm-registry", flag.ExitOnError)
	flagSet.StringVar(
		&outputFile,
		"output-file",
		"",
		"output file name to write config map to.",
	)
	flagSet.StringVar(
		&inputConfigMapFile,
		"input-configmap-file",
		"",
		"input configmap file to create the mindthegap repo file from",
	)
	flagSet.StringVar(
		&previousConfigMapFile,
		"previous-configmap-file",
		"",
		"input configmap file to create the mindthegap repo file from",
	)
	flagSet.StringVar(
		&nminus2ConfigMapFile,
		"n-minus-2-configmap-file",
		"",
		"input configmap file to create the mindthegap repo file from",
	)
	err := flagSet.Parse(args[1:])
	if err != nil {
		log.Error(err, "failed to parse args")
	}
	inputCm, err := getConfigMapFromFile(inputConfigMapFile)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to get configmap from file %s %w", inputConfigMapFile, err))
	}
	out := HelmChartsConfig{
		map[string]Repository{},
	}
	ConfigMapToHelmChartConfig(&out, inputCm)
	previousCm, err := getConfigMapFromFile(previousConfigMapFile)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to get configmap from file %s %w", inputConfigMapFile, err))
	}
	ConfigMapToHelmChartConfig(&out, previousCm)
	nMinus2Cm, err := getConfigMapFromFile(nminus2ConfigMapFile)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to get configmap from file %s %w", inputConfigMapFile, err))
	}
	ConfigMapToHelmChartConfig(&out, nMinus2Cm)
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
	f, err := os.OpenFile(fullOutputfilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		log.Error(err, "failed to create file")
	}
	defer f.Close()
	_, err = bytes.NewBuffer(b).WriteTo(f)
	if err != nil {
		log.Error(err, "failed to write to file")
	}
}

func ConfigMapToHelmChartConfig(out *HelmChartsConfig, cm *corev1.ConfigMap) {
	for _, info := range cm.Data {
		var settings HelmChartFromConfigMap
		err := yaml.Unmarshal([]byte(info), &settings)
		if err != nil {
			log.Error(err, "failed unmarshl settings")
			return
		}
		repo, ok := out.Repositories[settings.Name]
		// if this is the first time we saw this add a new entry
		if !ok {
			out.Repositories[settings.Name] = Repository{
				RepoURL: settings.Repository,
				Charts: map[string][]string{
					settings.Name: {
						settings.Version,
					},
				},
			}
			continue
		}
		// we've seen it already only add a new chart if the versions are different
		if !slices.Contains(repo.Charts[settings.Name], settings.Version) {
			repo.Charts[settings.Name] = append(repo.Charts[settings.Name], settings.Version)
		}
	}
}

func getConfigMapFromFile(configMapFile string) (*corev1.ConfigMap, error) {
	fullPath, err := EnsureFullPath(configMapFile)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cm := &corev1.ConfigMap{}
	err = yamlDecode.NewYAMLOrJSONDecoder(f, 1024).Decode(cm)
	return cm, err
}

func EnsureFullPath(filename string) (string, error) {
	fullPath, err := filepath.Abs(filename)
	if err != nil {
		return "", err
	}
	_, err = os.Stat(fullPath)
	if err != nil {
		return "", err
	}
	return fullPath, nil
}
