// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"
	"text/template"

	"github.com/d2iq-labs/helm-list-images/pkg"
	"github.com/d2iq-labs/helm-list-images/pkg/k8s"
	yamlv2 "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	createImagesCMD          = "create-images"
	defaultHelmAddonFilename = "values-template.yaml"
)

type ChartInfo struct {
	repo         string
	name         string
	valuesFile   string
	stringValues []string
}

func main() {
	args := os.Args
	var (
		chartDirectory     string
		helmChartConfigMap string
		carenVersion       string
	)
	flagSet := flag.NewFlagSet(createImagesCMD, flag.ExitOnError)
	flagSet.StringVar(&chartDirectory, "chart-directory", "",
		"path to chart directory for CAREN")
	flagSet.StringVar(&helmChartConfigMap, "helm-chart-configmap", "",
		"path to chart directory for CAREN")
	flagSet.StringVar(&carenVersion, "caren-version", "",
		"caren version for images override")
	err := flagSet.Parse(args[1:])
	if err != nil {
		fmt.Println("failed to parse args", err.Error())
		os.Exit(1)
	}
	chartDirectory, err = EnsureFullPath(chartDirectory)
	if err != nil {
		fmt.Println("failed to get full path for chart", err.Error())
		os.Exit(1)
	}
	helmChartConfigMap, err = EnsureFullPath(helmChartConfigMap)
	if err != nil {
		fmt.Println("failed to get full path for helmChartConfigMap", err.Error())
		os.Exit(1)
	}
	if chartDirectory == "" || helmChartConfigMap == "" {
		fmt.Println("chart-directory helm-chart-configmap must be set")
		os.Exit(1)
	}
	i := &ChartInfo{
		name: chartDirectory,
	}
	if carenVersion != "" {
		i.stringValues = []string{
			fmt.Sprintf("image.tag=%s", carenVersion),
			fmt.Sprintf("helmRepositoryImage.tag=%s", carenVersion),
		}
	}
	images, err := getImagesForChart(i)
	if err != nil {
		fmt.Println("failed to get images", err.Error())
		os.Exit(1)
	}
	addonImages, err := getImagesForAddons(helmChartConfigMap, chartDirectory)
	if err != nil {
		fmt.Println("failed to get images from addons", err.Error())
		os.Exit(1)
	}
	images = append(images, addonImages...)
	images = slices.Compact(images)
	for _, image := range images {
		fmt.Println(image)
	}
}

func EnsureFullPath(filename string) (string, error) {
	if path.IsAbs(filename) {
		return filename, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get wd: %w", err)
	}
	fullPath := path.Join(wd, filename)
	fullPath = path.Clean(fullPath)
	_, err = os.Stat(fullPath)
	if err != nil {
		return "", err
	}
	return fullPath, nil
}

type HelmChartFromConfigMap struct {
	Version    string `yaml:"ChartVersion"`
	Repository string `yaml:"RepositoryURL"`
	ChartName  string `yaml:"ChartName"`
}

func getImagesForAddons(helmChartConfigMap, carenChartDirectory string) ([]string, error) {
	templatedConfigMap, err := getTemplatedHelmConfigMap(helmChartConfigMap)
	if err != nil {
		return nil, err
	}
	cm := &corev1.ConfigMap{}
	err = yaml.Unmarshal([]byte(templatedConfigMap), cm)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configmap to object %w", err)
	}
	images := []string{}
	for _, chartInfoRaw := range cm.Data {
		var settings HelmChartFromConfigMap
		err = yamlv2.Unmarshal([]byte(chartInfoRaw), &settings)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal chart info from configmap %w", err)
		}
		info := &ChartInfo{
			name: settings.ChartName,
			repo: settings.Repository,
		}
		valuesFile := getValuesFileForChartIfNeeded(settings.ChartName, carenChartDirectory)
		if valuesFile != "" {
			info.valuesFile = valuesFile
		}
		if settings.ChartName == "aws-cloud-controller-manager" {
			values, err := getHelmValues(carenChartDirectory)
			if err != nil {
				return nil, err
			}
			awsImages, found, err := unstructured.NestedStringMap(values, "hooks", "ccm", "aws", "k8sMinorVersionToCCMVersion")
			if !found {
				return images, fmt.Errorf("failed to find k8sMinorVersionToCCMVersion from file %s",
					path.Join(carenChartDirectory, "values.yaml"))
			}
			if err != nil {
				return images, fmt.Errorf("failed to get map k8sMinorVersionToCCMVersion with error %w",
					err)
			}
			for _, tag := range awsImages {
				info.stringValues = []string{
					fmt.Sprintf("image.tag=%s", tag),
				}
				chartImages, err := getImagesForChart(info)
				if err != nil {
					return nil, fmt.Errorf("failed to get images for %s with error %w", info.name, err)
				}
				images = append(images, chartImages...)
			}
			// skip the to next addon because we got what we needed
			continue
		}
		chartImages, err := getImagesForChart(info)
		if err != nil {
			return nil, fmt.Errorf("failed to get images for %s with error %w", info.name, err)
		}
		images = append(images, chartImages...)
	}
	return images, nil
}

func getHelmValues(carenChartDirectory string) (map[string]interface{}, error) {
	values := path.Join(carenChartDirectory, "values.yaml")
	valuesFile, err := os.Open(values)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s with %w", values, err)
	}
	defer valuesFile.Close()
	m := make(map[string]interface{})
	err = yaml.NewYAMLOrJSONDecoder(valuesFile, 1024).Decode(&m)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s with %w", values, err)
	}
	return m, nil
}

func getValuesFileForChartIfNeeded(chartName, carenChartDirectory string) string {
	switch chartName {
	case "nutanix-csi-storage":
		return path.Join(carenChartDirectory, "addons", "csi", "nutanix", defaultHelmAddonFilename)
	case "node-feature-discovery":
		return path.Join(carenChartDirectory, "addons", "nfd", defaultHelmAddonFilename)
	case "snapshot-controller":
		return path.Join(carenChartDirectory, "addons", "csi", "snapshot-controller", defaultHelmAddonFilename)
	case "cilium":
		return path.Join(carenChartDirectory, "addons", "cni", "cilium", defaultHelmAddonFilename)
	// this uses the values from kustomize because the file at addons/cluster-autoscaler/values-template.yaml
	// is a file that is templated
	case "cluster-autoscaler":
		f := path.Join(carenChartDirectory, "addons", "cluster-autoscaler", defaultHelmAddonFilename)
		tempFile, _ := os.CreateTemp("", "")
		c := clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tmplCluster",
				Namespace: "templNamespace",
				Annotations: map[string]string{
					"caren.nutanix.com/cluster-uuid": "03c19062-493d-4c41-89c3-728624313118",
				},
			},
		}
		type input struct {
			Cluster *clusterv1.Cluster
		}

		templateInput := input{
			Cluster: &c,
		}

		template.Must(template.New(defaultHelmAddonFilename).ParseFiles(f)).Execute(tempFile, &templateInput)
		return tempFile.Name()
	default:
		return ""
	}
}

func getImagesForChart(info *ChartInfo) ([]string, error) {
	images := pkg.Images{}
	images.SetChart(info.name)
	if info.repo != "" {
		images.RepoURL = info.repo
	}
	if info.valuesFile != "" {
		images.ValueFiles.Set(info.valuesFile)
	}
	if len(info.stringValues) > 0 {
		images.StringValues = info.stringValues
	}
	// kubeVersion needs to be set for some addons
	images.KubeVersion = "v1.29.0"
	images.SetRelease("sample")
	images.SetLogger("INFO")
	images.Kind = k8s.SupportedKinds()
	b := bytes.NewBuffer([]byte{})
	images.SetWriter(b)
	images.ImageRegex = pkg.ImageRegex
	err := images.GetImages()
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(b)
	if err != nil {
		return nil, err
	}
	ret := strings.Split(string(raw), "\n")
	// this skips the last new line.
	ret = ret[0 : len(ret)-1]
	return ret, nil
}

func getTemplatedHelmConfigMap(helmChartConfigMap string) (string, error) {
	helmFile, err := os.Open(helmChartConfigMap)
	if err != nil {
		return "", fmt.Errorf("failed to parse files %w", err)
	}
	defer helmFile.Close()
	helmFileBytes, err := io.ReadAll(helmFile)
	if err != nil {
		return "", fmt.Errorf("failed to read file %w", err)
	}
	t, err := template.New(helmChartConfigMap).Parse(string(helmFileBytes))
	if err != nil {
		return "", fmt.Errorf("failed to parse files %w", err)
	}
	var b bytes.Buffer
	err = t.Execute(&b, nil)
	if err != nil {
		return "", fmt.Errorf("failed to execute template %w", err)
	}
	return b.String(), nil
}
