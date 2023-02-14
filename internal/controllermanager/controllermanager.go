// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controllermanager

import (
	"context"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	clusteraddonsv1alpha1 "github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/internal/controllers"
)

//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces;configmaps;secrets,verbs=watch;list;get;create;patch;update;delete

type Manager struct {
	port                 uint16
	webhookCertDir       string
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
}

func New() *Manager {
	return &Manager{
		port:                 8443,
		webhookCertDir:       "/controller-webhooks-certs/",
		metricsAddr:          ":8080",
		probeAddr:            ":8081",
		enableLeaderElection: false,
	}
}

func (m *Manager) AddFlags(prefix string, fs *pflag.FlagSet) {
	fs.Uint16Var(&m.port, prefix+".port", m.port, "The address the metric endpoint binds to.")

	fs.StringVar(&m.webhookCertDir, prefix+".cert-dir", m.webhookCertDir,
		"Controller webhook server cert dir.")

	fs.StringVar(&m.metricsAddr, prefix+".metrics-bind-address", m.metricsAddr,
		"The address the metric endpoint binds to.")

	fs.StringVar(&m.probeAddr, prefix+".health-probe-bind-address", m.probeAddr,
		"The address the probe endpoint binds to.")

	fs.BoolVar(&m.enableLeaderElection, prefix+".leader-elect", m.enableLeaderElection,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
}

func (m *Manager) Start(ctx context.Context) error {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusteraddonsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(capiv1beta1.AddToScheme(scheme))

	setupLog := ctrl.Log.WithName("controllers")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            m.metricsAddr,
		Port:                          int(m.port),
		HealthProbeBindAddress:        m.probeAddr,
		LeaderElection:                m.enableLeaderElection,
		LeaderElectionID:              clusteraddonsv1alpha1.GroupVersion.Group,
		LeaderElectionReleaseOnCancel: true,
		CertDir:                       m.webhookCertDir,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		return err
	}

	if err = (&controllers.ClusterAddonSetReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterAddonSet")
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}
