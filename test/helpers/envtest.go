// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package helpers provides a set of utilities for testing controllers.
package helpers

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	_      context.CancelFunc
}

// Cleanup deletes all the given objects.
func (t *TestEnvironment) Cleanup(ctx context.Context, objs ...client.Object) error {
	errs := []error{}
	for _, o := range objs {
		err := t.Client.Delete(ctx, o)
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
	if err := t.Client.Create(ctx, ns); err != nil {
		return nil, err
	}

	return ns, nil
}

// NewTestEnvironmentConfiguration creates a new test environment configuration for running tests.
func NewTestEnvironmentConfiguration() *TestEnvironmentConfiguration {
	return &TestEnvironmentConfiguration{
		env: &envtest.Environment{
			ErrorIfCRDPathMissing: true,
			CRDDirectoryPaths:     getFilePathsToCAPICRDs(),
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
// If a test is writeing an object, they are not immediately available to read since controller caches
// are not synchronized yet.
func (t *TestEnvironment) GetK8sClient() (client.Client, error) {
	return client.New(t.Manager.GetConfig(), client.Options{Scheme: scheme.Scheme})
}

// StartManager starts the test controller against the local API server.
func (t *TestEnvironment) StartManager(ctx context.Context) error {
	return t.Manager.Start(ctx)
}

// Stop stops the test environment.
func (t *TestEnvironment) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	return t.env.Stop()
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
			getModulePath(
				filepath.Join(rootDir(), "hack", "third-party", "capd"),
				"sigs.k8s.io/cluster-api/test",
			),
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(out))
}
