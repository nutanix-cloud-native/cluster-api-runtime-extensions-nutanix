// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file is a copy of the original file from the cluster-api repository, to allow resolving latest patch releases
// versions. Once this is released upstream in CPAI v1.7, this file can be removed and code switched to use the upstream
// config loader.

package clusterctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blang/semver/v4"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/yaml"

	"github.com/d2iq-labs/capi-runtime-extensions/test/framework/goproxy"
)

// LoadE2EConfig loads the configuration for the e2e test environment.
func LoadE2EConfig(ctx context.Context, input clusterctl.LoadE2EConfigInput) *clusterctl.E2EConfig {
	configData, err := os.ReadFile(input.ConfigPath)
	Expect(err).ToNot(HaveOccurred(), "Failed to read the e2e test config file")
	Expect(configData).ToNot(BeEmpty(), "The e2e test config file should not be empty")

	config := &clusterctl.E2EConfig{}
	Expect(
		yaml.Unmarshal(configData, config),
	).To(Succeed(), "Failed to convert the e2e test config file to yaml")

	Expect(
		ResolveReleases(ctx, config),
	).To(Succeed(), "Failed to resolve release markers in e2e test config file")
	config.Defaults()
	config.AbsPaths(filepath.Dir(input.ConfigPath))

	Expect(config.Validate()).To(Succeed(), "The e2e test config file is not valid")

	return config
}

// ResolveReleases converts release markers to release version.
func ResolveReleases(ctx context.Context, config *clusterctl.E2EConfig) error {
	for i := range config.Providers {
		provider := &config.Providers[i]
		for j := range provider.Versions {
			version := &provider.Versions[j]
			if version.Type != clusterctl.URLSource {
				continue
			}
			// Skipping versions that are not a resolvable marker. Resolvable markers are surrounded by `{}`
			if !strings.HasPrefix(version.Name, "{") || !strings.HasSuffix(version.Name, "}") {
				continue
			}
			releaseMarker := strings.TrimLeft(strings.TrimRight(version.Name, "}"), "{")
			ver, err := ResolveRelease(ctx, releaseMarker)
			if err != nil {
				return fmt.Errorf("failed resolving release url %q: %w", version.Name, err)
			}
			ver = "v" + ver
			version.Value = strings.Replace(version.Value, version.Name, ver, 1)
			version.Name = ver
		}
	}
	return nil
}

func ResolveRelease(ctx context.Context, releaseMarker string) (string, error) {
	scheme, host, err := goproxy.GetSchemeAndHost(os.Getenv("GOPROXY"))
	if err != nil {
		return "", err
	}
	if scheme == "" || host == "" {
		return "", fmt.Errorf(
			"releasemarker does not support disabling the go proxy: GOPROXY=%q",
			os.Getenv("GOPROXY"),
		)
	}
	goproxyClient := goproxy.NewClient(scheme, host)
	return resolveReleaseMarker(ctx, releaseMarker, goproxyClient)
}

// resolveReleaseMarker resolves releaseMarker string to verion string e.g.
// - Resolves "go://sigs.k8s.io/cluster-api@v1.0" to the latest stable patch release of v1.0.
// - Resolves "go://sigs.k8s.io/cluster-api@latest-v1.0" to the latest patch release of v1.0 including rc and
// pre releases.
func resolveReleaseMarker(
	ctx context.Context,
	releaseMarker string,
	goproxyClient *goproxy.Client,
) (string, error) {
	if !strings.HasPrefix(releaseMarker, "go://") {
		return "", errors.New("unknown release marker scheme")
	}

	releaseMarker = strings.TrimPrefix(releaseMarker, "go://")
	if releaseMarker == "" {
		return "", errors.New("empty release url")
	}

	gomoduleParts := strings.Split(releaseMarker, "@")
	if len(gomoduleParts) < 2 {
		return "", errors.New("go module or version missing")
	}
	gomodule := gomoduleParts[0]

	includePrereleases := false
	if strings.HasPrefix(gomoduleParts[1], "latest-") {
		includePrereleases = true
	}
	version := strings.TrimPrefix(gomoduleParts[1], "latest-") + ".0"
	version = strings.TrimPrefix(version, "v")
	semVersion, err := semver.Parse(version)
	if err != nil {
		return "", fmt.Errorf("parsing semver for %s: %w", version, err)
	}

	parsedTags, err := goproxyClient.GetVersions(ctx, gomodule)
	if err != nil {
		return "", err
	}

	var picked semver.Version
	for i, tag := range parsedTags {
		if !includePrereleases && len(tag.Pre) > 0 {
			continue
		}
		if tag.Major == semVersion.Major && tag.Minor == semVersion.Minor {
			picked = parsedTags[i]
		}
	}
	if picked.Major == 0 && picked.Minor == 0 && picked.Patch == 0 {
		return "", fmt.Errorf(
			"no suitable release available for release marker %s",
			releaseMarker,
		)
	}
	return picked.String(), nil
}
