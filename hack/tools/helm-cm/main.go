// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	yamlMarshal "sigs.k8s.io/yaml"
)

const (
	createHelmAddonsConfigMap = "helm-addons"
)

var log = ctrl.LoggerFrom(context.Background())

func main() {
	args := os.Args
	var (
		kustomizeDirectory string
		outputFile         string
	)
	flagSet := flag.NewFlagSet(createHelmAddonsConfigMap, flag.ExitOnError)
	flagSet.StringVar(&kustomizeDirectory, "kustomize-directory", "",
		"Kustomize base directory for all addons")
	flagSet.StringVar(&outputFile, "output-file", "",
		"output file name to write config map to.")
	err := flagSet.Parse(args[1:])
	if err != nil {
		log.Error(err, "failed to parse args")
	}
	cm, err := createConfigMapFromDir(kustomizeDirectory)
	if err != nil {
		log.Error(err, "failed to create configMap")
		return
	}
	b, err := yamlMarshal.Marshal(*cm)
	if err != nil {
		log.Error(err, "failed ")
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

type configMapInfo struct {
	configMapFieldName string
	RepositoryURL      string `json:"RepositoryURL"`
	ChartVersion       string `json:"ChartVersion"`
	ChartName          string `json:"ChartName"`
}

func createConfigMapFromDir(kustomizeDir string) (*corev1.ConfigMap, error) {
	fullPath := kustomizeDir
	if !path.IsAbs(fullPath) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get wd %w", err)
		}
		fullPath = path.Join(wd, kustomizeDir)
	}
	configDirFS := os.DirFS(fullPath)
	results := []configMapInfo{}
	err := fs.WalkDir(configDirFS, ".", func(filepath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(filepath, "kustomization.yaml.tmpl") && !isIgnored(filepath) {
			f, err := os.Open(path.Join(fullPath, filepath))
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()
			obj := make(map[string]interface{})
			err = yaml.NewYAMLOrJSONDecoder(f, 1024).Decode(&obj)
			if err != nil {
				return err
			}
			charts, ok := obj["helmCharts"]
			if !ok {
				log.Info("obj %v does not have field helmCharts. skipping \n", obj)
				return nil
			}
			parsedCharts, ok := charts.([]interface{})
			if !ok {
				return fmt.Errorf("charts obj %v is not of type []interface", charts)
			}
			info, ok := parsedCharts[0].(map[string]interface{})
			if !ok {
				return fmt.Errorf("info obj %v is not of type map[string]interface", parsedCharts)
			}
			repo := info["repo"].(string)
			name := info["name"].(string)
			dirName := strings.Split(filepath, "/")[0]
			i := configMapInfo{
				configMapFieldName: dirName,
				RepositoryURL:      repo,
				ChartName:          name,
			}
			versionEnvVar := info["version"].(string)
			version := os.ExpandEnv(versionEnvVar)
			i.ChartVersion = version
			results = append(results, i)
			return nil
		}
		return nil
	})

	finalCM := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "placeholder",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		Data: make(map[string]string),
	}
	for _, res := range results {
		d, err := yamlMarshal.Marshal(res)
		if err != nil {
			return &finalCM, err
		}
		finalCM.Data[res.configMapFieldName] = string(d)
	}
	return &finalCM, err
}

var ignored = []string{
	"aws-ccm",
	"aws-ebs-csi",
}

func isIgnored(filepath string) bool {
	for _, i := range ignored {
		if strings.Contains(filepath, i) {
			return true
		}
	}
	return false
}
