// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	artifactDir       = "_artifacts"
	extensionYAML     = ".yaml"
	extensionYML      = ".yml"
	kindKey           = "kind"
	secretKind        = "Secret"
	dataKey           = "data"
	stringDataKey     = "stringData"
	redactedValue     = "***REDACTED***"
	documentSeparator = "---"
)

func main() {
	err := filepath.Walk(artifactDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && isYAMLFile(path) {
			processYAMLFile(path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error walking the path: %v\n", err)
	}
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == extensionYAML || ext == extensionYML
}

func processYAMLFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Failed to read file %s: %v\n", path, err)
		return
	}

	output := make([]string, 0)
	// Split the file into multiple documents
	docs := strings.Split(string(data), documentSeparator)
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var unstructured map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &unstructured); err != nil {
			fmt.Printf("Failed to unmarshal document in file %s: %v\n", path, err)
			continue
		}

		if kind, ok := unstructured[kindKey].(string); ok && kind == secretKind {
			redactSecret(unstructured)
		}

		redacted, err := yaml.Marshal(unstructured)
		if err != nil {
			fmt.Printf("Failed to marshal document in file %s: %v\n", path, err)
			continue
		}

		output = append(output, string(redacted))
	}

	if err := os.WriteFile(path, []byte(strings.Join(output, "\n"+documentSeparator+"\n")), 0o600); err != nil {
		fmt.Printf("Failed to write file %s: %v\n", path, err)
	}
}

func redactSecret(unstructured map[string]interface{}) {
	if data, ok := unstructured[dataKey].(map[string]interface{}); ok {
		for key := range data {
			data[key] = redactedValue
		}
	}

	if stringData, ok := unstructured[stringDataKey].(map[string]interface{}); ok {
		for key := range stringData {
			stringData[key] = redactedValue
		}
	}
}
