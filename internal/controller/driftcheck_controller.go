package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	myv1 "github.com/somaz94/kube-drift/api/v1alpha1"
)

// DriftCheckReconciler reconciles a DriftCheck object.
type DriftCheckReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=drift.somaz.io,resources=driftchecks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=drift.somaz.io,resources=driftchecks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=drift.somaz.io,resources=driftchecks/finalizers,verbs=update

// Reconcile handles DriftCheck reconciliation logic.
func (r *DriftCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var resource myv1.DriftCheck
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling", "name", resource.Name, "namespace", resource.Namespace)

	// TODO: Add your reconciliation logic here.

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DriftCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&myv1.DriftCheck{}).
		Complete(r)
}
