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
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	createImagesCMD          = "create-images"
	defaultHelmAddonFilename = "values-template.yaml"
)

type ChartInfo struct {
	repo       string
	name       string
	valuesFile string
}

func main() {
	args := os.Args
	var (
		outputFile         string
		chartDirectory     string
		helmChartConfigMap string
	)
	flagSet := flag.NewFlagSet(createImagesCMD, flag.ExitOnError)
	flagSet.StringVar(&outputFile, "output-file", "",
		"output file name to write config map to.")
	flagSet.StringVar(&chartDirectory, "chart-directory", "",
		"path to chart directory for CAREN")
	flagSet.StringVar(&helmChartConfigMap, "helm-chart-configmap", "",
		"path to chart directory for CAREN")
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
	images, err := getImagesForChart(&ChartInfo{
		name: chartDirectory,
	})
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
	for chartName, chartInfoRaw := range cm.Data {
		var settings HelmChartFromConfigMap
		err = yamlv2.Unmarshal([]byte(chartInfoRaw), &settings)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal chart info from configmap %w", err)
		}
		info := &ChartInfo{
			name: chartName,
			repo: settings.Repository,
		}
		valuesFile := getValuesFileForChartIfNeeded(chartName, carenChartDirectory)
		if valuesFile != "" {
			info.valuesFile = valuesFile
		}
		chartImages, err := getImagesForChart(info)
		if err != nil {
			return nil, fmt.Errorf("failed to get images for %s with error %w", info.name, err)
		}
		images = append(images, chartImages...)
	}
	return images, nil
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
	return ret[0 : len(ret)-1], nil
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
