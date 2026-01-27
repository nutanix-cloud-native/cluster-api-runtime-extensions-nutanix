// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package enforceclusterautoscalerlimits

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/envtest"
)

var (
	ctx        = ctrl.SetupSignalHandler()
	fakeScheme = runtime.NewScheme()
	env        *envtest.Environment
)

func TestMain(m *testing.M) {
	_ = clientgoscheme.AddToScheme(fakeScheme)
	_ = clusterv1.AddToScheme(fakeScheme)
	_ = apiextensionsv1.AddToScheme(fakeScheme)
	_ = clusterv1.AddToScheme(fakeScheme)
	_ = corev1.AddToScheme(fakeScheme)

	setupReconcilers := func(ctx context.Context, mgr ctrl.Manager) {
		if err := (&Reconciler{
			Client: mgr.GetClient(),
		}).SetupWithManager(mgr, &controller.Options{MaxConcurrentReconciles: 1}); err != nil {
			panic(fmt.Sprintf("unable to create reconciler: %v", err))
		}
	}
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyTimeout(30 * time.Second)
	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M: m,
		SetupEnv: func(e *envtest.Environment) {
			env = e
		},
		SetupReconcilers: setupReconcilers,
	}))
}
