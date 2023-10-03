// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"testing"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	calicotests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/cni/calico/tests"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/region"
	regiontests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/region/tests"
	auditpolicytests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/auditpolicy/tests"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/cni"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/etcd"
	etcdtests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/etcd/tests"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/extraapiservercertsans"
	extraapiservercertsanstests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/extraapiservercertsans/tests"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/httpproxy"
	httpproxytests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/httpproxy/tests"
	imageregistrycredentials "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries/credentials"
	imageregistrycredentialstests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries/credentials/tests"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/kubernetesimagerepository"
	kubernetesimagerepositorytests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/kubernetesimagerepository/tests"
)

func metaPatchGeneratorFunc(mgr manager.Manager) func() mutation.GeneratePatches {
	return func() mutation.GeneratePatches {
		return MetaPatchHandler(mgr).(mutation.GeneratePatches)
	}
}

func TestGeneratePatches(t *testing.T) {
	t.Parallel()

	mgr, _ := manager.New(
		&rest.Config{},
		manager.Options{
			NewClient: func(_ *rest.Config, _ client.Options) (client.Client, error) {
				return fake.NewClientBuilder().Build(), nil
			},
		},
	)

	regiontests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		"clusterConfig",
		region.VariableName,
	)

	auditpolicytests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
	)

	httpproxytests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		"clusterConfig",
		httpproxy.VariableName,
	)

	etcdtests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		"clusterConfig",
		etcd.VariableName,
	)

	extraapiservercertsanstests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		"clusterConfig",
		extraapiservercertsans.VariableName,
	)

	kubernetesimagerepositorytests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		"clusterConfig",
		kubernetesimagerepository.VariableName,
	)

	imageregistrycredentialstests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		mgr.GetClient(),
		"clusterConfig",
		"imageRegistries",
		imageregistrycredentials.VariableName,
	)

	calicotests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		"clusterConfig",
		cni.VariableName,
	)
}
