// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	"github.com/d2iq-labs/helm-list-images/pkg"
	"github.com/d2iq-labs/helm-list-images/pkg/k8s"
	yamlv2 "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	createImagesCMD          = "create-images"
	defaultHelmAddonFilename = "values-template.yaml"
)

type ChartInfo struct {
	repo             string
	name             string
	version          string
	valuesFile       string
	stringValues     []string
	extraImagesFiles []string
}

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	args := os.Args
	var (
		chartDirectory      string
		helmChartConfigMap  string
		carenVersion        string
		additionalYAMLFiles stringSlice
	)
	flagSet := flag.NewFlagSet(createImagesCMD, flag.ExitOnError)
	flagSet.StringVar(&chartDirectory, "chart-directory", "",
		"path to chart directory for CAREN")
	flagSet.StringVar(&helmChartConfigMap, "helm-chart-configmap", "",
		"path to helm chart configmap for CAREN")
	flagSet.StringVar(&carenVersion, "caren-version", "",
		"CAREN version for images override")
	flagSet.Var(&additionalYAMLFiles, "additional-yaml-files",
		"additional YAML images to include")
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
		fmt.Println("chart-directory and helm-chart-configmap must be set")
		os.Exit(1)
	}
	i := &ChartInfo{
		name: chartDirectory,
	}
	if carenVersion != "" {
		i.stringValues = []string{
			fmt.Sprintf("image.tag=%s", carenVersion),
			fmt.Sprintf("helmRepository.images.bundleInitializer.tag=%s", carenVersion),
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
	additionalYAMLImages, err := getImagesFromYAMLFiles(additionalYAMLFiles)
	if err != nil {
		fmt.Println("failed to get images from additional YAML files", err.Error())
		os.Exit(1)
	}
	images = append(images, additionalYAMLImages...)
	slices.Sort(images)
	images = slices.Compact(images)
	for _, image := range images {
		fmt.Println(image)
	}
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
	var images []string
	for _, chartInfoRaw := range cm.Data {
		var settings HelmChartFromConfigMap
		err = yamlv2.Unmarshal([]byte(chartInfoRaw), &settings)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal chart info from configmap %w", err)
		}
		info := &ChartInfo{
			name:    settings.ChartName,
			version: settings.Version,
			repo:    settings.Repository,
		}
		valuesFile, err := getValuesFileForChartIfNeeded(settings.ChartName, carenChartDirectory)
		if err != nil {
			return nil, fmt.Errorf("failed to get values file for %s: %w", settings.ChartName, err)
		}
		if valuesFile != "" {
			info.valuesFile = valuesFile
		}

		switch settings.ChartName {
		case "aws-cloud-controller-manager":
			values, err := getHelmValues(carenChartDirectory)
			if err != nil {
				return nil, err
			}
			awsImages, found, err := unstructured.NestedStringMap(values, "hooks", "ccm", "aws", "k8sMinorVersionToCCMVersion")
			if !found {
				return images, fmt.Errorf("failed to find k8sMinorVersionToCCMVersion from file %s",
					filepath.Join(carenChartDirectory, "values.yaml"))
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
			// skip to the next addon because we got what we needed
			continue
		case "tigera-operator":
			extraImagesFile, err := os.CreateTemp("", "")
			if err != nil {
				return nil, fmt.Errorf("failed to create temp file for extra Calico images: %w", err)
			}
			defer os.Remove(extraImagesFile.Name()) //nolint:gocritic // Won't be leaked.
			_, err = extraImagesFile.WriteString(`
{{default "docker.io/" .Values.installation.registry }}calico/cni:{{ .Chart.Version }}
{{default "docker.io/" .Values.installation.registry }}calico/kube-controllers:{{ .Chart.Version }}
{{default "docker.io/" .Values.installation.registry }}calico/node:{{ .Chart.Version }}
{{default "docker.io/" .Values.installation.registry }}calico/apiserver:{{ .Chart.Version }}
{{default "docker.io/" .Values.installation.registry }}calico/pod2daemon-flexvol:{{ .Chart.Version }}
{{default "docker.io/" .Values.installation.registry }}calico/typha:{{ .Chart.Version }}
{{default "docker.io/" .Values.installation.registry }}calico/csi:{{ .Chart.Version }}
{{default "docker.io/" .Values.installation.registry }}calico/node-driver-registrar:{{ .Chart.Version }}
{{default "docker.io/" .Values.installation.registry }}calico/ctl:{{ .Chart.Version }}
`)
			_ = extraImagesFile.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to write to temp file for extra Calico images: %w", err)
			}

			info.extraImagesFiles = append(info.extraImagesFiles, extraImagesFile.Name())
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
	values := filepath.Join(carenChartDirectory, "values.yaml")
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

func getValuesFileForChartIfNeeded(chartName, carenChartDirectory string) (string, error) {
	switch chartName {
	case "nutanix-csi-storage":
		return filepath.Join(carenChartDirectory, "addons", "csi", "nutanix", defaultHelmAddonFilename), nil
	case "node-feature-discovery":
		return filepath.Join(carenChartDirectory, "addons", "nfd", defaultHelmAddonFilename), nil
	case "snapshot-controller":
		return filepath.Join(carenChartDirectory, "addons", "csi", "snapshot-controller", defaultHelmAddonFilename), nil
	case "cilium":
		return filepath.Join(carenChartDirectory, "addons", "cni", "cilium", defaultHelmAddonFilename), nil
	// Calico values differ slightly per provider, but that does not have a material imapct on the images required
	// so we can use the default values file for AWS provider.
	case "tigera-operator":
		f := filepath.Join(carenChartDirectory, "addons", "cni", "calico", "aws", defaultHelmAddonFilename)
		tempFile, err := os.CreateTemp("", "")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}

		// CAAPH uses unstructured internally, so we need to create an unstructured copy of a cluster
		// to pass to the CAAPH values template.
		c, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&clusterv1.Cluster{})
		if err != nil {
			return "", fmt.Errorf("failed to convert cluster to unstructured %w", err)
		}

		templateInput := struct {
			Cluster map[string]interface{}
		}{
			Cluster: c,
		}

		err = template.Must(template.New(defaultHelmAddonFilename).ParseFiles(f)).Execute(tempFile, &templateInput)
		if err != nil {
			return "", fmt.Errorf("failed to execute helm values template %w", err)
		}

		return tempFile.Name(), nil
	// this uses the values from kustomize because the file at addons/cluster-autoscaler/values-template.yaml
	// is a file that is templated
	case "cluster-autoscaler":
		f := filepath.Join(carenChartDirectory, "addons", "cluster-autoscaler", defaultHelmAddonFilename)
		tempFile, err := os.CreateTemp("", "")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}

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

		err = template.Must(template.New(defaultHelmAddonFilename).ParseFiles(f)).Execute(tempFile, &templateInput)
		if err != nil {
			return "", fmt.Errorf("failed to execute helm values template %w", err)
		}

		return tempFile.Name(), nil
	case "cosi-controller":
		return filepath.Join(carenChartDirectory, "addons", "cosi", "controller", defaultHelmAddonFilename), nil
	default:
		return "", nil
	}
}

func getImagesForChart(info *ChartInfo) ([]string, error) {
	images := pkg.Images{}
	images.SetChart(info.name)
	images.ChartVersionConstraint = info.version
	images.RepoURL = info.repo
	if info.valuesFile != "" {
		_ = images.ValueFiles.Set(info.valuesFile)
	}
	images.StringValues = info.stringValues
	images.ExtraImagesFiles = info.extraImagesFiles
	// kubeVersion needs to be set for some addons
	images.KubeVersion = "v1.29.0"
	// apiVersions needs to be set for some addons
	images.APIVersions = []string{"snapshot.storage.k8s.io/v1"}
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

func getImagesFromYAMLFiles(files []string) ([]string, error) {
	var images []string
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s with %w", f, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, " image: ") {
				// Get everything after "image: "
				image := strings.SplitAfterN(line, "image: ", 2)
				if len(image) == 2 {
					images = append(images, strings.TrimSpace(image[1]))
				}
			}
		}

		err = scanner.Err()
		if err != nil {
			return nil, fmt.Errorf("failed to scan file %s: %w", f, err)
		}
	}
	return images, nil
}
