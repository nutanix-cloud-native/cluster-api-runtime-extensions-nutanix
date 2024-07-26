// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
)

const (
	syncHelmValues          = "sync-helm-values"
	chartTemplateFile       = "-template.yaml"
	helmValuesConfigMapName = "helm-addon-installation.yaml"
)

func main() {
	var (
		kustomizeDir string
		helmChartDir string
		licenseFile  string
	)
	args := os.Args
	flagSet := flag.NewFlagSet(
		syncHelmValues,
		flag.ExitOnError,
	)
	flagSet.StringVar(
		&kustomizeDir,
		"kustomize-directory",
		"",
		"Kustomize base directory for all addons",
	)
	flagSet.StringVar(
		&helmChartDir,
		"helm-chart-directory",
		"",
		"Directory of all the helm chart",
	)
	flagSet.StringVar(
		&licenseFile,
		"license-file",
		"",
		"License file for templating",
	)
	err := flagSet.Parse(args[1:])
	if err != nil {
		fmt.Println("failed to parse args", err.Error())
		os.Exit(1)
	}
	kustomizeDir, err = EnsureFullPath(kustomizeDir)
	if err != nil {
		fmt.Println("failed to ensure full path for argument", err.Error())
		os.Exit(1)
	}
	helmChartDir, err = EnsureFullPath(helmChartDir)
	if err != nil {
		fmt.Println("failed to ensure full path for argument", err.Error())
		os.Exit(1)
	}
	licenseFile, err = EnsureFullPath(licenseFile)
	if err != nil {
		fmt.Println("failed to ensure full path for argument", err.Error())
		os.Exit(1)
	}
	if kustomizeDir == "" || helmChartDir == "" {
		fmt.Println("-helm-chart-directory and -kustomize-directory must be set")
		os.Exit(1)
	}
	err = SyncHelmValues(kustomizeDir, helmChartDir, licenseFile)
	if err != nil {
		fmt.Println("failed to sync err:", err.Error())
		os.Exit(1)
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

func SyncHelmValues(sourceDirectory, destDirectory, licenseFile string) error {
	sourceFS := os.DirFS(sourceDirectory)
	err := fs.WalkDir(sourceFS, ".", func(filepath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// we skip tigera because we don't use that with CAAPH
		// we generate ClusterResourceSets with this instead
		if strings.Contains(filepath, "tigera") {
			return nil
		}
		if !(strings.Contains(filepath, helmValuesConfigMapName) || strings.Contains(filepath, chartTemplateFile)) {
			return nil
		}
		destPath := getDestPath(destDirectory, filepath)
		out, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s with error %w", destPath, err)
		}
		defer out.Close()
		srcPath := path.Join(sourceDirectory, filepath)
		in, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open file %s with error %w", srcPath, err)
		}
		defer in.Close()
		inBytes, err := io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("failed to read all bytes of %s %w", in.Name(), err)
		}
		if !strings.Contains(filepath, chartTemplateFile) {
			license, err := os.Open(licenseFile)
			if err != nil {
				return fmt.Errorf("failed to open license file %s with error %w", licenseFile, err)
			}
			defer license.Close()
			_, err = license.WriteTo(out)
			if err != nil {
				return fmt.Errorf("failed to write license to file %s %w", out.Name(), err)
			}
		}
		cleanedString := sanitizeSourceValues(string(inBytes))
		b := bytes.NewBufferString(cleanedString)
		_, err = b.WriteTo(out)
		if err != nil {
			return fmt.Errorf("failed to write to file %s %w", out.Name(), err)
		}
		return nil
	})
	return err
}

func getDestPath(destDirectory, filepath string) string {
	if strings.Contains(filepath, chartTemplateFile) {
		filepath = strings.Replace(filepath, "manifests", "", 1)
		return path.Clean(path.Join(destDirectory, "addons", filepath))
	}
	return path.Clean(path.Join(destDirectory, "templates", filepath))
}

func sanitizeSourceValues(sourceString string) string {
	// we template this with environment variables when using CRS
	// expand it out.
	sourceString = strings.ReplaceAll(
		sourceString,
		"${NODE_FEATURE_DISCOVERY_VERSION}",
		os.Getenv("NODE_FEATURE_DISCOVERY_VERSION"),
	)
	// remove the license
	pattern := `(?m)^# Copyright 202(\d+) Nutanix\. All rights reserved\.\n?`
	re := regexp.MustCompile(pattern)
	sourceString = re.ReplaceAllString(sourceString, "")

	pattern = `# SPDX-License-Identifier: Apache-2.0\n?`
	re = regexp.MustCompile(pattern)
	return re.ReplaceAllString(sourceString, "")
}
