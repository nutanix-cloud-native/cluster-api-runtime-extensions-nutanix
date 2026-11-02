// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package golden

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	carenhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers"
)

var updateGolden = flag.String("update-golden", "",
	"Write missing golden files for every handler, and overwrite existing ones for handlers of this version (e.g. v7)")

var providers = []string{"aws", "docker", "eks", "nutanix", "generic"}

var (
	handlerVersion   = regexp.MustCompile(`v([0-9]+)(configpatch)?$`)
	discoveryVersion = regexp.MustCompile(`v[0-9]+$`)
)

func registeredHandlers(t *testing.T) []handlers.Named {
	t.Helper()
	return carenhandlers.AllHandlers(testEnv.Manager)
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

// discoveryFor returns the discovery handlers for the ClusterClass patches in the input at path, keyed by patch name.
// Whatever version the input's ClusterClass names, a patch handler of version vN pairs with the vN discovery handlers,
// or with the unversioned ones if vN has none.
func discoveryFor(
	t *testing.T, all []handlers.Named, path, version string,
) map[string]mutation.DiscoverVariables {
	t.Helper()
	in, err := LoadInput(path)
	require.NoError(t, err)
	discovery := map[string]mutation.DiscoverVariables{}
	for _, p := range in.ClusterClass.Spec.Patches {
		if p.External == nil || p.External.DiscoverVariablesExtension == "" {
			continue
		}
		extension, _, _ := strings.Cut(p.External.DiscoverVariablesExtension, ".")
		base := discoveryVersion.ReplaceAllString(strings.TrimSuffix(extension, "-dv"), "")
		find := func(name string) int {
			return slices.IndexFunc(all, func(h handlers.Named) bool { return strings.ToLower(h.Name()) == name })
		}
		i := find(base + version)
		if i < 0 {
			i = find(base)
		}
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

// inputsFor returns the corpus inputs a handler of provider runs against; generic handlers run against all of them.
func inputsFor(t *testing.T, provider string) []string {
	t.Helper()
	dir := provider
	if provider == "generic" {
		dir = "*"
	}
	inputs, err := filepath.Glob(filepath.Join("testdata", "corpus", dir, "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, inputs)
	return inputs
}

func patchFor(name string) string {
	if strings.Contains(name, "Worker") {
		return "worker-config"
	}
	return "cluster-config"
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
		patch := patchFor(h.Name())
		for _, path := range inputsFor(t, provider) {
			inputProvider := filepath.Base(filepath.Dir(path))
			input := strings.TrimSuffix(filepath.Base(path), ".yaml")
			t.Run(h.Name()+"/"+inputProvider+"/"+input, func(t *testing.T) {
				t.Parallel()
				discovery := discoveryFor(t, all, path, versionOf(h.Name()))
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

// uncoverable lists mutators, by type or by provider/type, that cannot change any template a ClusterClass of that
// provider can contain.
var uncoverable = map[string]string{
	"docker/*calico.calicoPatchHandler":  "only patches AWSClusterTemplate",
	"nutanix/*calico.calicoPatchHandler": "only patches AWSClusterTemplate",
}

// TestGoldenMutatorCoverage fails if a mutator changes no template for any corpus input on its own, because golden
// files would then not notice that mutator being dropped from a handler.
func TestGoldenMutatorCoverage(t *testing.T) {
	t.Parallel()

	all := registeredHandlers(t)
	for _, h := range all {
		meta, ok := h.(interface{ Mutators() []mutation.MetaMutator })
		if !ok {
			continue
		}
		provider := providerOf(h.Name())
		patch := patchFor(h.Name())
		inputs := inputsFor(t, provider)
		for i, m := range meta.Mutators() {
			t.Run(fmt.Sprintf("%s/%d-%T", h.Name(), i, m), func(t *testing.T) {
				t.Parallel()
				typ := fmt.Sprintf("%T", m)
				for _, key := range []string{typ, provider + "/" + typ} {
					if reason, ok := uncoverable[key]; ok {
						t.Skip(reason)
					}
				}
				only := mutation.NewMetaGeneratePatchesHandler(h.Name(), testEnv.Client, m).(mutation.GeneratePatches)
				none := mutation.NewMetaGeneratePatchesHandler(h.Name(), testEnv.Client).(mutation.GeneratePatches)
				for _, path := range inputs {
					discovery := discoveryFor(t, all, path, versionOf(h.Name()))
					run := func(g mutation.GeneratePatches) []byte {
						in, err := LoadInput(path)
						require.NoError(t, err)
						out, err := Run(t.Context(), testEnv.Client, g, in, patch, discovery)
						require.NoError(t, err)
						return out
					}
					if got := run(only); !bytes.HasPrefix(got, []byte("error:")) && !bytes.Equal(got, run(none)) {
						return
					}
				}
				t.Errorf("no corpus input is changed by this mutator; add one to hack/handlers that enables it")
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

// TestGoldenLatestVersionServesAPI checks that the highest handler version serves the current API schema. Once that
// version is released, an API change makes this fail until a new handler version is cut.
func TestGoldenLatestVersionServesAPI(t *testing.T) {
	t.Parallel()

	dirs, err := filepath.Glob("../v[0-9]*")
	require.NoError(t, err)
	latest := slices.MaxFunc(dirs, func(a, b string) int {
		var x, y int
		_, _ = fmt.Sscanf(filepath.Base(a), "v%d", &x)
		_, _ = fmt.Sscanf(filepath.Base(b), "v%d", &y)
		return x - y
	})

	want, err := filepath.Glob("../../../api/v1alpha1/crds/*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, want)
	got, err := filepath.Glob(filepath.Join(latest, "variables", "crds", "*.yaml"))
	require.NoError(t, err)
	base := func(paths []string) []string {
		out := make([]string, 0, len(paths))
		for _, p := range paths {
			out = append(out, filepath.Base(p))
		}
		return out
	}
	require.Equal(t, base(want), base(got), "CRD snapshot of %s is out of date; run make go-generate", latest)
	for _, w := range want {
		wantBytes, err := os.ReadFile(w)
		require.NoError(t, err)
		gotBytes, err := os.ReadFile(filepath.Join(latest, "variables", "crds", filepath.Base(w)))
		require.NoError(t, err)
		assert.Equal(t, string(wantBytes), string(gotBytes),
			"CRD snapshot %s of %s is out of date; run make go-generate", filepath.Base(w), latest)
	}
}
