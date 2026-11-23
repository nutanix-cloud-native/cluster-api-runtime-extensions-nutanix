// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallbconfig

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	metallbv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/go.universe.tf/metallb/api/v1beta1"
	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/envtest"
)

var (
	ctx = ctrl.SetupSignalHandler()
	env *envtest.Environment
)

func TestMain(m *testing.M) {
	utilruntime.Must(caaphv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(metallbv1.AddToScheme(scheme.Scheme))

	setupReconcilers := func(_ context.Context, mgr ctrl.Manager) {
		if err := (&Reconciler{
			Client: mgr.GetClient(),
		}).SetupWithManager(mgr, &controller.Options{MaxConcurrentReconciles: 1}); err != nil {
			panic(fmt.Sprintf("unable to create reconciler: %v", err))
		}
	}

	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyTimeout(15 * time.Second)
	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M: m,
		EnvironmentOpts: []envtest.EnvironmentOpt{
			envtest.WithCRDDirectoryPaths(
				envtest.ThirdPartyCRDDirectoryPath(
					"caaph",
					"sigs.k8s.io/cluster-api-addon-provider-helm",
					"config",
					"crd",
					"bases",
				),
				envtest.ThirdPartyCRDDirectoryPath(
					"metallb",
					"go.universe.tf/metallb",
					"config",
					"crd",
					"bases",
				),
			),
		},
		SetupEnv: func(e *envtest.Environment) {
			env = e
		},
		SetupReconcilers: setupReconcilers,
	}))
}
