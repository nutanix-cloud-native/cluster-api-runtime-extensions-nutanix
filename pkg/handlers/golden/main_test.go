// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package golden

import (
	"context"
	"fmt"
	"os"
	"testing"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	crsv1 "sigs.k8s.io/cluster-api/api/addons/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var testEnv *helpers.TestEnvironment

func TestMain(m *testing.M) {
	utilruntime.Must(crsv1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(controlplanev1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(caaphv1.AddToScheme(clientgoscheme.Scheme))
	utilruntime.Must(capxv1.AddToScheme(clientgoscheme.Scheme))

	var err error
	testEnv, err = helpers.NewTestEnvironmentConfiguration().Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := testEnv.StartManager(ctx); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	code := m.Run()

	cancel()
	if err := testEnv.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
