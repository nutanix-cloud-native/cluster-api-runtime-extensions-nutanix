// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/ami"
	amitests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/ami/tests"
	calicotests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/cni/calico/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/controlplaneloadbalancer"
	controlplaneloadbalancertests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/controlplaneloadbalancer/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/iaminstanceprofile"
	iaminstanceprofiletests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/iaminstanceprofile/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/instancetype"
	instancetypetests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/instancetype/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/network"
	networktests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/network/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/region"
	regiontests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/region/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	auditpolicytests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/auditpolicy/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/etcd"
	etcdtests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/etcd/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/extraapiservercertsans"
	extraapiservercertsanstests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/extraapiservercertsans/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/httpproxy"
	httpproxytests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/httpproxy/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/imageregistries"
	imageregistrycredentialstests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/imageregistries/credentials/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubernetesimagerepository"
	kubernetesimagerepositorytests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubernetesimagerepository/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/mirrors"
	globalimageregistrymirrortests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/mirrors/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/users"
	userstests "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/users/tests"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/workerconfig"
)

func metaPatchGeneratorFunc(mgr manager.Manager) func() mutation.GeneratePatches {
	return func() mutation.GeneratePatches {
		return MetaPatchHandler(mgr).(mutation.GeneratePatches)
	}
}

func workerPatchGeneratorFunc() func() mutation.GeneratePatches {
	return func() mutation.GeneratePatches {
		return MetaWorkerPatchHandler().(mutation.GeneratePatches)
	}
}

func TestGeneratePatches(t *testing.T) {
	t.Parallel()

	mgr := testEnv.Manager

	regiontests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		v1alpha1.AWSVariableName,
		region.VariableName,
	)

	iaminstanceprofiletests.TestControlPlaneGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		clusterconfig.MetaControlPlaneConfigName,
		v1alpha1.AWSVariableName,
		iaminstanceprofile.VariableName,
	)

	iaminstanceprofiletests.TestWorkerGeneratePatches(
		t,
		workerPatchGeneratorFunc(),
		workerconfig.MetaVariableName,
		v1alpha1.AWSVariableName,
		iaminstanceprofile.VariableName,
	)

	instancetypetests.TestControlPlaneGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		clusterconfig.MetaControlPlaneConfigName,
		v1alpha1.AWSVariableName,
		instancetype.VariableName,
	)

	instancetypetests.TestWorkerGeneratePatches(
		t,
		workerPatchGeneratorFunc(),
		workerconfig.MetaVariableName,
		v1alpha1.AWSVariableName,
		instancetype.VariableName,
	)

	calicotests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		"addons",
		v1alpha1.CNIVariableName,
	)

	auditpolicytests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
	)

	httpproxytests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		httpproxy.VariableName,
	)

	etcdtests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		etcd.VariableName,
	)

	extraapiservercertsanstests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		extraapiservercertsans.VariableName,
	)

	kubernetesimagerepositorytests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		kubernetesimagerepository.VariableName,
	)

	imageregistrycredentialstests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		mgr.GetClient(),
		clusterconfig.MetaVariableName,
		imageregistries.VariableName,
	)

	globalimageregistrymirrortests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		mgr.GetClient(),
		clusterconfig.MetaVariableName,
		mirrors.GlobalMirrorVariableName,
	)

	amitests.TestControlPlaneGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		clusterconfig.MetaControlPlaneConfigName,
		v1alpha1.AWSVariableName,
		ami.VariableName,
	)

	amitests.TestWorkerGeneratePatches(
		t,
		workerPatchGeneratorFunc(),
		workerconfig.MetaVariableName,
		v1alpha1.AWSVariableName,
		ami.VariableName,
	)

	networktests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		v1alpha1.AWSVariableName,
		network.VariableName,
	)

	controlplaneloadbalancertests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		v1alpha1.AWSVariableName,
		controlplaneloadbalancer.VariableName,
	)

	userstests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		clusterconfig.MetaVariableName,
		users.VariableName,
	)
}
