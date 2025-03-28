package credentials

import (
	"context"
	"errors"
	"fmt"
	"time"

	credsv1alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// CredentialsRequestReconciler reconciles a CredentialsRequest object
type CredentialsRequestReconciler struct {
	client.Client
}

// Reconcile handles the credential rotation logic
func (r *CredentialsRequestReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	var credRequest credsv1alpha1.CredentialsRequest
	if err := r.Get(ctx, req.NamespacedName, &credRequest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("Reconciling credentials request", "Name", credRequest.Name)

	// find the referenced cluster
	cluster := &clusterv1.Cluster{}
	if err := r.Get(ctx,
		client.ObjectKey{
			Name:      credRequest.Spec.ClusterRef.Name,
			Namespace: credRequest.Spec.ClusterRef.Namespace,
		},
		cluster); err != nil {
		if errors.Is(err, client.IgnoreNotFound(err)) {
			log.Info(
				"referenced cluster not found or created yet",
				"Name",
				credRequest.Spec.ClusterRef.Name,
			)
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		log.Info("failed to get cluster", "Name", credRequest.Spec.ClusterRef.Name, "Error", err)
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}
	err := r.reconcileCredentialsSecret(ctx, &credRequest, cluster)
	if err != nil {
		log.Info("failed to reconcile credentials", credRequest.Name, err)
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	// regular forced reconciliation
	return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
}

// reconcileCredentialsSecret creates or updates secret with credentials for a cluster
func (r *CredentialsRequestReconciler) reconcileCredentialsSecret(
	ctx context.Context,
	credRequest *credsv1alpha1.CredentialsRequest,
	cluster *clusterv1.Cluster,
) error {
	log := ctrl.LoggerFrom(ctx)

	switch credRequest.Spec.Component {
	case credsv1alpha1.ComponentNutanixCluster:
		log.Info("creating secret", "SecretName", credRequest.Spec.SecretRef.Name)
		err := r.reconcileNutanixClusterCredentials(ctx, credRequest, cluster)
		if err != nil {
			return err
		}
	case credsv1alpha1.ComponentNutanixCCM:
		log.Info("creating secret", "SecretName", credRequest.Spec.SecretRef.Name)
		err := r.reconcileNutanixCCMCredentials(ctx, credRequest, cluster)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported credential component: %s", credRequest.Spec.Component)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CredentialsRequestReconciler) SetupWithManager(
	ctx context.Context,
	mgr ctrl.Manager,
	options *controller.Options,
) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&credsv1alpha1.CredentialsRequest{}).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretToCredentialsRequest),
			builder.WithPredicates(rootSecretPredicates(ctx))).
		Named("credentialsrequest-controller").
		WithOptions(*options).
		Complete(r)
}

func (r *CredentialsRequestReconciler) secretToCredentialsRequest(
	ctx context.Context,
	o client.Object,
) []ctrl.Request {
	log := ctrl.LoggerFrom(ctx)
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(o), secret); err != nil {
		log.Error(err, "Failed to get root secret object: %s", o.GetName())
		return nil
	}
	result := []ctrl.Request{}
	for _, ownerRef := range secret.GetOwnerReferences() {
		if ownerRef.Kind == "CredentialsRequest" {

			name := client.ObjectKey{
				Namespace: o.GetNamespace(),
				Name:      ownerRef.Name,
			}
			result = append(result, ctrl.Request{NamespacedName: name})
		}
	}
	return result
}

func rootSecretPredicates(ctx context.Context) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret, ok := e.Object.(*corev1.Secret)
			if !ok {
				return false
			}
			return SecretHasCredentialsType(secret)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newSecret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}
			resourceVersionChangedPredicate := predicate.ResourceVersionChangedPredicate{}
			if !resourceVersionChangedPredicate.Update(e) {
				return false
			}
			return SecretHasCredentialsType(newSecret)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// if root secret is deleted, it should not affect management or workload clusters
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func SecretHasCredentialsType(secret *corev1.Secret) bool {
	return secret.Type == credsv1alpha1.CredentialsSecretType
}
