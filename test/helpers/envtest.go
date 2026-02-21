// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package helpers provides a set of utilities for testing controllers.
package helpers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	root     string
	rootOnce sync.Once
)

func rootDir() string {
	rootOnce.Do(func() {
		// Get the root of the current file to use in CRD paths.
		_, filename, _, _ := goruntime.Caller(0) //nolint:dogsled // Just want the filename.
		root, _ = filepath.Abs(filepath.Join(filepath.Dir(filename), "..", ".."))
	})
	return root
}

type TestEnvironmentConfiguration struct {
	env *envtest.Environment
}

// TestEnvironment encapsulates a Kubernetes local test environment.
type TestEnvironment struct {
	manager.Manager
	client.Client
	Config *rest.Config
	env    *envtest.Environment
	cancel context.CancelFunc
}

// Cleanup deletes all the given objects.
func (t *TestEnvironment) Cleanup(ctx context.Context, objs ...client.Object) error {
	errs := []error{}
	for _, o := range objs {
		err := t.Delete(ctx, o)
		if apierrors.IsNotFound(err) {
			continue
		}
		errs = append(errs, err)
	}
	return kerrors.NewAggregate(errs)
}

// CreateNamespace creates a new namespace with a generated name.
func (t *TestEnvironment) CreateNamespace(
	ctx context.Context,
	generateName string,
) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", generateName),
			Labels: map[string]string{
				"testenv/original-name": generateName,
			},
		},
	}
	if err := t.Create(ctx, ns); err != nil {
		return nil, err
	}

	return ns, nil
}

// NewTestEnvironmentConfiguration creates a new test environment configuration for running tests.
func NewTestEnvironmentConfiguration() *TestEnvironmentConfiguration {
	return &TestEnvironmentConfiguration{
		env: &envtest.Environment{
			ErrorIfCRDPathMissing: true,
			CRDs:                  loadCRDsWithV1Beta2Storage(getFilePathsToCAPICRDs()),
		},
	}
}

// Build creates a new environment spinning up a local api-server.
// This function should be called only once for each package you're running tests within,
// usually the environment is initialized in a suite_test.go file within a `BeforeSuite` ginkgo block.
func (t *TestEnvironmentConfiguration) Build() (*TestEnvironment, error) {
	if _, err := t.env.Start(); err != nil {
		return nil, err
	}

	options := manager.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	}

	mgr, err := ctrl.NewManager(t.env.Config, options)
	if err != nil {
		klog.Fatalf("Failed to start testenv manager: %v", err)
		if stopErr := t.env.Stop(); stopErr != nil {
			err = errors.Join(err, stopErr)
		}
		return nil, err
	}

	return &TestEnvironment{
		Manager: mgr,
		Client:  mgr.GetClient(),
		Config:  mgr.GetConfig(),
		env:     t.env,
	}, nil
}

// GetK8sClient returns a “live” k8s client that does will not invoke against controller cache.
// If a test is writing an object, they are not immediately available to read since controller caches
// are not synchronized yet.
func (t *TestEnvironment) GetK8sClient() (client.Client, error) {
	return client.New(t.GetConfig(), client.Options{Scheme: scheme.Scheme})
}

// GetK8sClientWithScheme - same as GetK8sClient but can pass in a configurable scheme.
func (t *TestEnvironment) GetK8sClientWithScheme(
	clientScheme *runtime.Scheme,
) (client.Client, error) {
	return client.New(t.GetConfig(), client.Options{Scheme: clientScheme})
}

// WithFakeRemoteClusterClient creates a fake remote cluster client Secret pointing to the test API server.
func (t *TestEnvironment) WithFakeRemoteClusterClient(cluster *clusterv1.Cluster) error {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	cfg := t.GetConfig()
	c, err := client.New(cfg, client.Options{Scheme: clientScheme})
	if err != nil {
		return err
	}

	v1beta2Cluster := &clusterv1.Cluster{
		ObjectMeta: *cluster.ObjectMeta.DeepCopy(),
	}
	kubeconfigBytes := kubeconfig.FromEnvTestConfig(cfg, v1beta2Cluster)
	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name(cluster.Name, secret.Kubeconfig),
			Namespace: cluster.Namespace,
		},
		Data: map[string][]byte{
			secret.KubeconfigDataName: kubeconfigBytes,
		},
		Type: clusterv1.ClusterSecretType,
	}
	err = controllerutil.SetOwnerReference(cluster, kubeconfigSecret, c.Scheme())
	if err != nil {
		return fmt.Errorf("failed to set cluster's owner reference on kubeconfig secret: %w", err)
	}

	return c.Create(context.Background(), kubeconfigSecret)
}

// StartManager starts the test controller against the local API server.
func (t *TestEnvironment) StartManager(ctx context.Context) error {
	return t.Start(ctx)
}

// Stop stops the test environment.
func (t *TestEnvironment) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	return t.env.Stop()
}

// loadCRDsWithV1Beta2Storage reads CRD files from directories and patches multi-version
// CRDs to use v1beta2 as the storage version. envtest lacks conversion webhooks, so only
// the storage version round-trips correctly. Since this project uses v1beta2 typed clients,
// v1beta2 must be the storage version.
func loadCRDsWithV1Beta2Storage(dirs []string) []*apiextensionsv1.CustomResourceDefinition {
	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			panic("failed to read CRD directory " + dir + ": " + err.Error())
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := filepath.Ext(entry.Name())
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				panic("failed to read CRD file " + entry.Name() + ": " + err.Error())
			}
			crd := &apiextensionsv1.CustomResourceDefinition{}
			if err := k8syaml.NewYAMLOrJSONDecoder(
				bytes.NewReader(data), 4096,
			).Decode(crd); err != nil {
				panic("failed to decode CRD from " + entry.Name() + ": " + err.Error())
			}
			forceV1Beta2Storage(crd)
			crds = append(crds, crd)
		}
	}
	return crds
}

func forceV1Beta2Storage(crd *apiextensionsv1.CustomResourceDefinition) {
	hasV1Beta2 := false
	for _, v := range crd.Spec.Versions {
		if v.Name == "v1beta2" {
			hasV1Beta2 = true
			break
		}
	}
	if !hasV1Beta2 {
		return
	}
	for i := range crd.Spec.Versions {
		crd.Spec.Versions[i].Storage = crd.Spec.Versions[i].Name == "v1beta2"
	}
}

func getFilePathsToCAPICRDs() []string {
	return []string{
		filepath.Join(
			getModulePath(rootDir(), "sigs.k8s.io/cluster-api"),
			"config",
			"crd",
			"bases",
		),
		filepath.Join(
			getModulePath(
				filepath.Join(rootDir(), "hack", "third-party", "capa"),
				"sigs.k8s.io/cluster-api-provider-aws/v2"),
			"config", "crd", "bases",
		),
		filepath.Join(
			getModulePath(rootDir(), "sigs.k8s.io/cluster-api/test"),
			"infrastructure",
			"docker",
			"config",
			"crd",
			"bases",
		),
		filepath.Join(
			getModulePath(
				filepath.Join(rootDir(), "hack", "third-party", "capx"),
				"github.com/nutanix-cloud-native/cluster-api-provider-nutanix",
			),
			"config", "crd", "bases",
		),
		filepath.Join(
			getModulePath(
				filepath.Join(rootDir(), "hack", "third-party", "caaph"),
				"sigs.k8s.io/cluster-api-addon-provider-helm",
			),
			"config", "crd", "bases",
		),
	}
}

func getModulePath(moduleDir, moduleName string) string {
	cmd := exec.Command("go", "list", "-m", "-f", "{{ .Dir }}", moduleName)
	cmd.Dir = moduleDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// We include the combined output because the error is usually
		// an exit code, which does not explain why the command failed.
		panic(
			fmt.Sprintf("cmd.Dir=%q, cmd.Args=%q, err=%q, output=%q",
				cmd.Dir,
				cmd.Args,
				err,
				out),
		)
	}
	return strings.TrimSpace(string(out))
}
