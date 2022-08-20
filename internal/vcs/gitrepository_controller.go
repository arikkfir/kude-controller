package vcs

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/arikkfir/kude/internal/vcs/v1alpha1"
)

// GitRepositoryReconciler reconciles a GitRepository object
type GitRepositoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=vcs.kude.kfirs.com,resources=gitrepositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vcs.kude.kfirs.com,resources=gitrepositories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vcs.kude.kfirs.com,resources=gitrepositories/finalizers,verbs=update

// Reconcile continuously aims to move the current state of [GitRepository] objects closer to their desired state.
func (r *GitRepositoryReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GitRepository{}).
		Complete(r)
}
