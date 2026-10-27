// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package golden

import (
	"bufio"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

var updateGolden = flag.String("update-golden", "",
	"Write missing golden files for every handler, and overwrite existing ones for handlers of this version (e.g. v7)")

var providers = []string{"aws", "docker", "eks", "nutanix", "generic"}

var handlerVersion = regexp.MustCompile(`v([0-9]+)configpatch$`)

func registeredHandlers(t *testing.T) []handlers.Named {
	t.Helper()
	globalOptions := options.NewGlobalOptions()
	flags := pflag.NewFlagSet("golden", pflag.ContinueOnError)
	globalOptions.AddFlags(flags)
	require.NoError(t, flags.Parse(nil))

	mgr := testEnv.Manager
	return slices.Concat(
		aws.New(globalOptions).AllHandlers(mgr),
		docker.New(globalOptions).AllHandlers(mgr),
		nutanix.New(globalOptions).AllHandlers(mgr),
		eks.New(globalOptions).AllHandlers(mgr),
		generic.New().AllHandlers(mgr),
	)
}

func providerOf(name string) string {
	lower := strings.ToLower(name)
	for _, p := range providers {
		if strings.HasPrefix(lower, p) {
			return p
		}
	}
	return ""
}

// discoveryFor returns the discovery handlers referenced by the ClusterClass in the input at path, keyed by patch name.
func discoveryFor(t *testing.T, all []handlers.Named, path string) map[string]mutation.DiscoverVariables {
	t.Helper()
	in, err := LoadInput(path)
	require.NoError(t, err)
	discovery := map[string]mutation.DiscoverVariables{}
	for _, p := range in.ClusterClass.Spec.Patches {
		if p.External == nil || p.External.DiscoverVariablesExtension == "" {
			continue
		}
		extension, _, _ := strings.Cut(p.External.DiscoverVariablesExtension, ".")
		i := slices.IndexFunc(all, func(h handlers.Named) bool {
			return strings.ToLower(h.Name())+"-dv" == extension
		})
		require.GreaterOrEqual(t, i, 0, "%s: no registered handler for %s", path, extension)
		discovery[p.Name] = all[i].(mutation.DiscoverVariables)
	}
	return discovery
}

func checkGolden(t *testing.T, name, path string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist) && *updateGolden != "":
		writeGolden(t, path, got)
		return
	case err != nil:
		require.NoError(t, err, "missing golden file; run with -update-golden=<open version>")
	}
	if !slices.Equal(want, got) && *updateGolden != "" && versionOf(name) == *updateGolden {
		writeGolden(t, path, got)
		return
	}
	assert.Equal(t, string(want), string(got), "output of %s changed", name)
}

func versionOf(name string) string {
	if m := handlerVersion.FindStringSubmatch(name); m != nil {
		return "v" + m[1]
	}
	return ""
}

func writeGolden(t *testing.T, path string, data []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func TestGolden(t *testing.T) {
	t.Parallel()

	all := registeredHandlers(t)
	inputs, err := filepath.Glob("testdata/corpus/*/*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, inputs)

	for _, h := range all {
		provider := providerOf(h.Name())
		require.NotEmpty(t, provider, "cannot tell provider of handler %s", h.Name())

		if d, ok := h.(mutation.DiscoverVariables); ok {
			t.Run(h.Name(), func(t *testing.T) {
				t.Parallel()
				resp := &runtimehooksv1.DiscoverVariablesResponse{}
				d.DiscoverVariables(t.Context(), &runtimehooksv1.DiscoverVariablesRequest{}, resp)
				got, err := yaml.Marshal(resp)
				require.NoError(t, err)
				checkGolden(t, h.Name(), filepath.Join("testdata", "golden", h.Name(), "variables.yaml"), got)
			})
		}

		g, ok := h.(mutation.GeneratePatches)
		if !ok {
			continue
		}
		patch := "cluster-config"
		if strings.Contains(h.Name(), "Worker") {
			patch = "worker-config"
		}
		for _, path := range inputs {
			inputProvider := filepath.Base(filepath.Dir(path))
			if provider != "generic" && provider != inputProvider {
				continue
			}
			input := strings.TrimSuffix(filepath.Base(path), ".yaml")
			t.Run(h.Name()+"/"+inputProvider+"/"+input, func(t *testing.T) {
				t.Parallel()
				discovery := discoveryFor(t, all, path)
				var outputs [][]byte
				for range 2 {
					in, err := LoadInput(path)
					require.NoError(t, err)
					out, err := Run(t.Context(), testEnv.Client, g, in, patch, discovery)
					require.NoError(t, err)
					outputs = append(outputs, out)
				}
				require.Equal(t, string(outputs[0]), string(outputs[1]), "output is not deterministic")
				checkGolden(t, h.Name(), filepath.Join("testdata", "golden", h.Name(), inputProvider, input+".yaml"),
					outputs[0])
			})
		}
	}
}

func TestGoldenHandlersRegistered(t *testing.T) {
	t.Parallel()

	known := map[string]bool{}
	for _, h := range registeredHandlers(t) {
		known[h.Name()] = true
	}
	removed, err := os.Open("../removed-handlers.txt")
	if err == nil {
		defer removed.Close()
		scanner := bufio.NewScanner(removed)
		for scanner.Scan() {
			if fields := strings.Fields(scanner.Text()); len(fields) > 0 && !strings.HasPrefix(fields[0], "#") {
				known[fields[0]] = true
			}
		}
		require.NoError(t, scanner.Err())
	} else {
		require.ErrorIs(t, err, os.ErrNotExist)
	}

	dirs, err := os.ReadDir("testdata/golden")
	require.NoError(t, err)
	for _, d := range dirs {
		assert.True(t, known[d.Name()],
			"golden files exist for %s, which is neither registered nor listed in removed-handlers.txt", d.Name())
	}
}
