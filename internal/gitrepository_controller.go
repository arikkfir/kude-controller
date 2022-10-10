package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"time"

	"github.com/arikkfir/kude-controller/internal/v1alpha1"
)

const (
	finalizerGitRepository     = "gitrepositories.kude.kfirs.com/finalizer"
	typeAvailableGitRepository = "Available" // Is the GitRepository available for applying by bundles
	typeClonedGitRepository    = "Cloned"    // Is the GitRepository cloned to the local filesystem
	typeDegradedGitRepository  = "Degraded"  // When the GitRepository is deleted, but finalizer not applied yet
)

// GitRepositoryReconciler reconciles a GitRepository object
type GitRepositoryReconciler struct {
	Client   client.Client        // Kubernetes API client
	Recorder record.EventRecorder // Kubernetes event recorder
	Scheme   *runtime.Scheme      // Scheme registry
	WorkDir  string               // Working directory for the controller
}

//+kubebuilder:rbac:groups=kude.kfirs.com,resources=gitrepositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=gitrepositories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=gitrepositories/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile continuously aims to move the current state of [GitRepository] objects closer to their desired state.
func (r *GitRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var o v1alpha1.GitRepository
	if err := r.Client.Get(ctx, req.NamespacedName, &o); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Ensure statuses have the "Unknown" value when they are missing
	for _, conditionType := range []string{typeAvailableGitRepository, typeClonedGitRepository} {
		if meta.FindStatusCondition(o.Status.Conditions, conditionType) == nil {
			if res, err := r.setCondition(ctx, &o, conditionType, metav1.ConditionUnknown, "Reconciling", "Initial value"); res.Requeue || err != nil {
				return res, err
			}
		}
	}

	// Add our finalizer
	if controllerutil.AddFinalizer(&o, finalizerGitRepository) {
		if err := r.Client.Update(ctx, &o); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update resource status with added finalizer: %w", err)
		} else {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// If marked for deletion, perform actual deletion & remove finalizer
	if o.DeletionTimestamp != nil {
		if res, err := r.setCondition(ctx, &o, typeDegradedGitRepository, metav1.ConditionTrue, "Deleted", "Deleting resource"); res.Requeue || err != nil {
			return ctrl.Result{}, err
		}
		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "Deleted", "Deleting resource"); res.Requeue || err != nil {
			return ctrl.Result{}, err
		}
		if o.Status.WorkDirectory != "" {
			if strings.HasPrefix(o.Status.WorkDirectory, r.WorkDir+"/") {
				if err := os.RemoveAll(o.Status.WorkDirectory); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to delete local clone: %w", err)
				}
				o.Status.WorkDirectory = ""
				if err := r.Client.Status().Update(ctx, &o); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to delete local clone: %w", err)
				} else {
					return ctrl.Result{Requeue: true}, nil
				}
			} else {
				r.Recorder.Eventf(&o, v1.EventTypeWarning, "InvalidWorkDirectory", "Work directory '%s' is not under %s/", o.Status.WorkDirectory, r.WorkDir)
				return ctrl.Result{Requeue: false}, nil
			}
		}
		if res, err := r.setCondition(ctx, &o, typeClonedGitRepository, metav1.ConditionFalse, "CloneDeleted", ""); res.Requeue || err != nil {
			return ctrl.Result{}, err
		}
		if controllerutil.RemoveFinalizer(&o, finalizerGitRepository) {
			if err := r.Client.Update(ctx, &o); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Set work path if missing/incorrect
	if o.Status.WorkDirectory != filepath.Join(r.WorkDir, string(o.UID)) {
		o.Status.WorkDirectory = filepath.Join(r.WorkDir, string(o.UID))
		if err := r.Client.Status().Update(ctx, &o); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update work directory in GitRepository status: %w", err)
		} else {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Get interval
	interval, err := time.ParseDuration(o.Spec.PollingInterval)
	if err != nil {
		if _, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "InvalidPollingInterval", "Invalid polling interval: "+o.Spec.PollingInterval); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: false}, nil
	}

	// Clone the repository if it's missing
	b := bytes.Buffer{}
	if _, err := os.Stat(o.Status.WorkDirectory); err != nil {

		// If the directory doesn't exist, clone the repository
		if errors.Is(err, os.ErrNotExist) {

			// Update the "Cloned" & "Available" conditions to "False"
			if res, err := r.setCondition(ctx, &o, typeClonedGitRepository, metav1.ConditionFalse, "NotCloned", ""); res.Requeue || err != nil {
				return res, err
			}
			if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "NotCloned", ""); res.Requeue || err != nil {
				return res, err
			}

			// No clone exists, update status to reflect we have no pulled SHA
			if o.Status.LastPulledSHA != "" {
				o.Status.LastPulledSHA = ""
				if err := r.Client.Status().Update(ctx, &o); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to update GitRepository status: %w", err)
				} else {
					// Reset our last-pulled SHA status; requeue now to clone the repository
					return ctrl.Result{Requeue: true}, nil
				}
			}

			// Clone
			cloneOptions := git.CloneOptions{URL: o.Spec.URL, ReferenceName: plumbing.ReferenceName(o.Spec.Branch), Progress: &b}
			if _, err := git.PlainClone(o.Status.WorkDirectory, false, &cloneOptions); err != nil {
				r.Recorder.Eventf(&o, v1.EventTypeWarning, "CloneFailed", "Failed to clone repository:\n%s", b.String())

				// Clone failed - remove partial directory (if any)
				if err := os.RemoveAll(o.Status.WorkDirectory); err != nil {
					r.Recorder.Eventf(&o, v1.EventTypeWarning, "CleanupError", "Failed to remove failed clone directory at '%s': %s", o.Status.WorkDirectory, err.Error())
				}

				// Retry on next tick
				return ctrl.Result{RequeueAfter: interval}, nil
			} else {
				// Requeue now (not next tick) in order to progress to the next phase (reading the repository)
				r.Recorder.Eventf(&o, v1.EventTypeNormal, "Cloned", "Cloned repository:\n%s", b.String())
				return ctrl.Result{Requeue: true}, nil
			}

		} else { // Unknown error

			// Unknown error while reading clone directory - update the "Cloned" to "Unknown", and the "Available" condition to "False"
			if res, err := r.setCondition(ctx, &o, typeClonedGitRepository, metav1.ConditionUnknown, "CloneInaccessible", "Failed to stat clone: "+err.Error()); res.Requeue || err != nil {
				return res, err
			}
			if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "CloneInaccessible", "Failed to stat clone: "+err.Error()); res.Requeue || err != nil {
				return res, err
			}

			// Retry on the next tick
			return ctrl.Result{RequeueAfter: interval}, nil
		}

	} else if repository, err := git.PlainOpen(o.Status.WorkDirectory); err != nil {

		// Failed to open repository; update the "Cloned" condition to "Unknown"
		if res, err := r.setCondition(ctx, &o, typeClonedGitRepository, metav1.ConditionUnknown, "CloneOpenFailed", "Clone open failed: "+err.Error()); res.Requeue || err != nil {
			return res, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil

	} else if res, err := r.setCondition(ctx, &o, typeClonedGitRepository, metav1.ConditionTrue, "Cloned", ""); res.Requeue || err != nil {

		// Ensure the "Cloned" condition is set to "True"
		return res, err

	} else if origin, err := repository.Remote("origin"); err != nil {

		// Ensure the "Available" condition is set to "False"
		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "RemoteLookupFailed", "Remote lookup failed: "+err.Error()); err != nil {
			return res, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil

	} else if len(origin.Config().URLs) != 1 {

		// Ensure the "Available" condition is set to "False"
		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "InvalidRemote", fmt.Sprintf("Expected 1 URL, found: %v", origin.Config().URLs)); err != nil {
			return res, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil

	} else if origin.Config().URLs[0] != o.Spec.URL {

		msg := fmt.Sprintf("URL changed from '%s' to '%s'", origin.Config().URLs[0], o.Spec.URL)
		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "URLChanged", msg); err != nil {
			return res, err
		}
		if res, err := r.setCondition(ctx, &o, typeClonedGitRepository, metav1.ConditionUnknown, "URLChanged", msg); err != nil {
			return res, err
		}
		if err := os.RemoveAll(o.Status.WorkDirectory); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed deleting clone: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil

	} else if err := origin.Fetch(&git.FetchOptions{Progress: &b, Tags: git.AllTags}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {

		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "RemoteFetchFailed", "Failed to fetch remote: "+err.Error()); err != nil {
			return res, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil

	} else if worktree, err := repository.Worktree(); err != nil {

		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "WorktreeReadFailed", "Failed to read worktree: "+err.Error()); err != nil {
			return res, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil

	} else if err := worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.ReferenceName(o.Spec.Branch), Force: true}); err != nil {

		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "CheckoutFailed", "Failed to checkout branch: "+err.Error()); err != nil {
			return res, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil

	} else if err := worktree.Pull(&git.PullOptions{Progress: &b}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {

		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "PullFailed", "Failed to pull branch: "+err.Error()); err != nil {
			return res, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil

	} else if head, err := repository.Head(); err != nil {

		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionFalse, "HeadReadFailed", "Failed to get HEAD reference: "+err.Error()); err != nil {
			return res, err
		}
		return ctrl.Result{RequeueAfter: interval}, nil

	} else if o.Status.LastPulledSHA != head.Hash().String() {

		o.Status.LastPulledSHA = head.Hash().String()
		if err := r.Client.Status().Update(ctx, &o); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		} else {
			return ctrl.Result{Requeue: true}, nil
		}

	} else if !meta.IsStatusConditionTrue(o.Status.Conditions, typeAvailableGitRepository) {

		if res, err := r.setCondition(ctx, &o, typeAvailableGitRepository, metav1.ConditionTrue, "Ready", ""); err != nil {
			return res, err
		} else {
			return ctrl.Result{Requeue: true}, nil
		}

	} else {

		return ctrl.Result{}, nil

	}
}

func (r *GitRepositoryReconciler) setCondition(ctx context.Context, o *v1alpha1.GitRepository, conditionType string, status metav1.ConditionStatus, reason, message string) (ctrl.Result, error) {
	if c := meta.FindStatusCondition(o.Status.Conditions, conditionType); c == nil {
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{
			Type:    conditionType,
			Status:  status,
			Reason:  reason,
			Message: message,
		})
		if err := r.Client.Status().Update(ctx, o); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add missing condition '%s=%s: %s': %w", conditionType, status, reason, err)
		} else {
			return ctrl.Result{Requeue: true}, nil
		}
	} else if c.Status != status || c.Reason != reason || c.Message != message {
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{
			Type:    conditionType,
			Status:  status,
			Reason:  reason,
			Message: message,
		})
		if err := r.Client.Status().Update(ctx, o); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update condition '%s=%s: %s': %w", conditionType, status, reason, err)
		} else {
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		return ctrl.Result{}, nil
	}
}

func (r *GitRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.Recorder = mgr.GetEventRecorderFor("gitrepository")
	r.Scheme = mgr.GetScheme()
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GitRepository{}).
		Complete(r)
}
