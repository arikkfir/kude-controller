package internal

import (
	"bytes"
	"context"
	"fmt"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/strings"
	"os/exec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// KubectlBundleReconciler reconciles a KubectlBundle object
type KubectlBundleReconciler struct {
	client.Client
	record.EventRecorder
	Scheme *runtime.Scheme
}

func (r *KubectlBundleReconciler) SetStatus(ctx context.Context, b *v1alpha1.KubectlBundle) {
	if err := r.Status().Update(ctx, b); err != nil {
		// TODO: keep retrying if we're just out of date
		r.Eventf(b, v1.EventTypeWarning, "StatusUpdateFailed", "Failed to update status: %v", err)
	}
}

//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles/finalizers,verbs=update

// Reconcile continuously aims to move the current state of [GitRepository] objects closer to their desired state.
func (r *KubectlBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var bundle v1alpha1.KubectlBundle
	if err := r.Get(ctx, req.NamespacedName, &bundle); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if bundle.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(&bundle, kudeFinalizerName) {
			controllerutil.AddFinalizer(&bundle, kudeFinalizerName)
			if err := r.Update(ctx, &bundle); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(&bundle, kudeFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			// TODO: prune objects created by the YAML manifests

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&bundle, kudeFinalizerName)
			if err := r.Update(ctx, &bundle); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	var repo v1alpha1.GitRepository
	gitRepoNamespace, gitRepoName := strings.SplitQualifiedName(bundle.Spec.SourceRepository)
	if err := r.Get(ctx, types.NamespacedName{Namespace: gitRepoNamespace, Name: gitRepoName}, &repo); err != nil {
		r.Eventf(&bundle, v1.EventTypeWarning, "GitRepositoryNotFound", "GitRepository '%s' not found", bundle.Spec.SourceRepository)
		return ctrl.Result{Requeue: false}, nil
	}

	if repo.Status.LastPulledSHA != "" && repo.Status.LastPulledSHA != bundle.Status.AppliedSHA {
		// Create command
		args := make([]string, 0)
		args = append(args, "apply", "--dry-run=client", "-f")
		args = append(args, bundle.Spec.Files...)
		cmd := exec.CommandContext(ctx, "kubectl", args...)
		cmd.Dir = repo.Status.WorkDirectory
		var b bytes.Buffer
		cmd.Stdout = &b
		cmd.Stderr = &b

		// Create KubectlRun object
		run := v1alpha1.KubectlRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      string(uuid.NewUUID()),
				Namespace: bundle.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					// Ensure this run is deleted when the bundle is deleted
					*metav1.NewControllerRef(&bundle, v1alpha1.GroupVersion.WithKind("KubectlBundle")),
				},
			},
			Spec: v1alpha1.KubectlRunSpec{
				Directory: cmd.Dir,
				Command:   cmd.Path,
				Args:      cmd.Args,
			},
		}
		if err := r.Create(ctx, &run); err != nil {
			return ctrl.Result{}, err
		}
		bundle.Status.AppliedSHA = repo.Status.LastPulledSHA
		r.SetStatus(ctx, &bundle)

		// Start the command
		if err := cmd.Start(); err != nil {
			run.Status.ExitCode = -1
			run.Status.Error = fmt.Errorf("failed to start command: %w", err).Error()
			if err := r.Status().Update(ctx, &run); err != nil {
				log.FromContext(ctx).Error(err, "Failed to update KubectlRun status")
			}
			return ctrl.Result{}, nil
		}
		r.Eventf(&bundle, v1.EventTypeNormal, "KubectlRunStarted", "Started kubectl run: kubectl %v", args)

		// Wait for the command to finish
		if err := cmd.Wait(); err != nil {
			run.Status.ExitCode = cmd.ProcessState.ExitCode()
			run.Status.Output = b.String()
			run.Status.Error = fmt.Errorf("command failed: %w", err).Error()
			r.Eventf(&bundle, v1.EventTypeWarning, "KubectlRunFailed", run.Status.Error)
			if err := r.Status().Update(ctx, &run); err != nil {
				log.FromContext(ctx).Error(err, "Failed to update KubectlRun status")
			}
			return ctrl.Result{}, nil
		}

		r.Eventf(&bundle, v1.EventTypeNormal, "KubectlRunFinished", "Finished kubectl run: kubectl %v", args)
		run.Status.ExitCode = cmd.ProcessState.ExitCode()
		run.Status.Output = b.String()
		// TODO: save list of objects to be pruned on delete in bundle status
		if err := r.Status().Update(ctx, &run); err != nil {
			log.FromContext(ctx).Error(err, "Failed to update KubectlRun status")
		}
	}

	return ctrl.Result{}, nil
}

func (r *KubectlBundleReconciler) findObjectsForGitRepository(gr client.Object) []reconcile.Request {
	grKey := gr.GetNamespace() + "/" + gr.GetName()

	bundles := &v1alpha1.KubectlBundleList{}
	listOps := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(".spec.sourceRepository", grKey),
	}
	err := r.List(context.TODO(), bundles, listOps)
	if err != nil {
		ctrl.Log.Error(err, "Failed listing Kubectl bundles for GitRepository", "gitRepository", grKey)
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, len(bundles.Items))
	for i, b := range bundles.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      b.GetName(),
				Namespace: b.GetNamespace(),
			},
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubectlBundleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha1.KubectlBundle{}, ".spec.sourceRepository", func(rawObj client.Object) []string {
		// Extract the ConfigMap name from the ConfigDeployment Spec, if one is provided
		bundle := rawObj.(*v1alpha1.KubectlBundle)
		if bundle.Spec.SourceRepository == "" {
			return nil
		}
		return []string{bundle.Spec.SourceRepository}
	}); err != nil {
		return fmt.Errorf("failed to create index for source-repository: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubectlBundle{}).
		Watches(
			&source.Kind{Type: &v1alpha1.GitRepository{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForGitRepository),
		).
		Complete(r)
}
