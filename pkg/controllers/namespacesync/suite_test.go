// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/envtest"
)

const (
	targetNamespaceLabelKey = "test"
)

var (
	ctx        = ctrl.SetupSignalHandler()
	fakeScheme = runtime.NewScheme()
	env        *envtest.Environment

	sourceClusterClassNamespace = "source"
)

func TestMain(m *testing.M) {
	_ = clientgoscheme.AddToScheme(fakeScheme)
	_ = clusterv1.AddToScheme(fakeScheme)
	_ = apiextensionsv1.AddToScheme(fakeScheme)
	_ = expv1.AddToScheme(fakeScheme)
	_ = corev1.AddToScheme(fakeScheme)

	setupReconcilers := func(ctx context.Context, mgr ctrl.Manager) {
		unstructuredCachingClient, err := client.New(mgr.GetConfig(), client.Options{
			Cache: &client.CacheOptions{
				Reader:       mgr.GetCache(),
				Unstructured: true,
			},
		})
		if err != nil {
			panic(fmt.Sprintf("unable to create unstructuredCachineClient: %v", err))
		}

		// Create a label selector that matches namespaces with the target label key
		targetSelector, err := metav1.ParseToLabelSelector(targetNamespaceLabelKey)
		if err != nil {
			panic(fmt.Sprintf("unable to parse label selector: %v", err))
		}
		targetLabelSelector, err := metav1.LabelSelectorAsSelector(targetSelector)
		if err != nil {
			panic(fmt.Sprintf("unable to convert label selector: %v", err))
		}

		if err := (&Reconciler{
			Client:                      mgr.GetClient(),
			UnstructuredCachingClient:   unstructuredCachingClient,
			SourceClusterClassNamespace: sourceClusterClassNamespace,
			TargetNamespaceSelector:     targetLabelSelector,
		}).SetupWithManager(mgr, &controller.Options{MaxConcurrentReconciles: 1}); err != nil {
			panic(fmt.Sprintf("unable to create reconciler: %v", err))
		}
	}
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyTimeout(30 * time.Second)
	os.Exit(envtest.Run(ctx, envtest.RunInput{
		M: m,
		SetupEnv: func(e *envtest.Environment) {
			err := e.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: sourceClusterClassNamespace,
				},
			})
			if err != nil {
				panic(fmt.Sprintf("unable to create source namespace: %s", err))
			}

			env = e
		},
		SetupReconcilers: setupReconcilers,
	}))
}
