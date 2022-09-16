package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
	"time"

	"github.com/arikkfir/kude-controller/internal/v1alpha1"
)

type trackingInfo struct {
	logger    logr.Logger
	interval  time.Duration
	ticker    *time.Ticker
	namespace string
	name      string
	path      string
	url       string
	branch    string
}

// GitRepositoryReconciler reconciles a GitRepository object
type GitRepositoryReconciler struct {
	client.Client        // Kubernetes API client
	record.EventRecorder // Kubernetes event recorder

	// TODO: use this as a lock to update trackers, and then a lock per repository
	sync.Locker                             // Synchronization mutex lock
	Scheme      *runtime.Scheme             // Scheme registry
	trackers    map[types.UID]*trackingInfo // Mapping of tracking info per GitRepository UID
}

//+kubebuilder:rbac:groups=kude.kfirs.com,resources=gitrepositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=gitrepositories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=gitrepositories/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;patch

// Reconcile continuously aims to move the current state of [GitRepository] objects closer to their desired state.
func (r *GitRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var repo v1alpha1.GitRepository
	if err := r.Get(ctx, req.NamespacedName, &repo); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// examine DeletionTimestamp to determine if object is under deletion
	if repo.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(&repo, kudeFinalizerName) {
			controllerutil.AddFinalizer(&repo, kudeFinalizerName)
			if err := r.Update(ctx, &repo); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(&repo, kudeFinalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.stopTracking(ctx, &repo); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&repo, kudeFinalizerName)
			if err := r.Update(ctx, &repo); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if err := r.track(ctx, &repo); err != nil {
		return ctrl.Result{Requeue: false}, err
	}

	return ctrl.Result{}, nil
}

func (r *GitRepositoryReconciler) track(ctx context.Context, gr *v1alpha1.GitRepository) error {
	r.Lock()
	defer r.Unlock()

	pi, err := time.ParseDuration(gr.Spec.PollingInterval)
	if err != nil {
		tracker, ok := r.trackers[gr.UID]
		if ok {
			tracker.ticker.Stop()
			delete(r.trackers, gr.UID)
		}
		msg := fmt.Sprintf("Failed to parse polling interval '%s': %v", gr.Spec.PollingInterval, err)
		r.Eventf(gr, v1.EventTypeWarning, "InvalidPollingInterval", msg)
		r.setStatusCondition(
			ctx,
			gr,
			metav1.Condition{Type: gitRepositoryTrackedCondition, Status: metav1.ConditionFalse, Reason: "InvalidPollingInterval", Message: msg},
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "InvalidPollingInterval", Message: msg},
		)
		return err
	}

	tracker, ok := r.trackers[gr.UID]
	if !ok {
		tracker = &trackingInfo{
			logger:    log.FromContext(ctx).WithName("ticker"),
			interval:  pi,
			ticker:    time.NewTicker(pi),
			namespace: gr.Namespace,
			name:      gr.Name,
			path:      filepath.Join("/data", string(gr.UID)),
			url:       gr.Spec.URL,
			branch:    gr.Spec.Branch,
		}
		r.trackers[gr.UID] = tracker

		go r.loop(gr.UID)
		r.Eventf(gr, v1.EventTypeNormal, "TrackingStarted", "Started tracking")
		r.setStatusCondition(
			ctx, gr,
			metav1.Condition{Type: gitRepositoryTrackedCondition, Status: metav1.ConditionTrue, Reason: "TrackingStarted"},
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "TrackingStarted"},
		)
	} else {
		if tracker.url != gr.Spec.URL {
			oldURL := tracker.url
			newURL := gr.Spec.URL
			tracker.url = newURL
			r.Eventf(gr, v1.EventTypeNormal, "TrackingURLUpdated", fmt.Sprintf("Tracker URL changed from '%s' to '%s'", oldURL, newURL))
		}
		if tracker.branch != gr.Spec.Branch {
			oldBranch := tracker.branch
			newBranch := gr.Spec.Branch
			tracker.branch = newBranch
			r.Eventf(gr, v1.EventTypeNormal, "TrackingBranchUpdated", fmt.Sprintf("Tracker branch changed from '%s' to '%s'", oldBranch, newBranch))
		}
		if pi != tracker.interval {
			oldInterval := tracker.interval
			newInterval := pi
			tracker.ticker.Reset(newInterval)
			r.Eventf(gr, v1.EventTypeNormal, "TrackingPollingIntervalUpdated", fmt.Sprintf("Polling interval changed from '%s' to '%s'", oldInterval, newInterval))
		}
	}
	return nil
}

func (r *GitRepositoryReconciler) stopTracking(ctx context.Context, gr *v1alpha1.GitRepository) error {
	r.Lock()
	defer r.Unlock()

	tracker, ok := r.trackers[gr.UID]
	if ok {
		tracker.ticker.Stop()
		delete(r.trackers, gr.UID)

		if err := os.RemoveAll(tracker.path); err != nil {
			tracker.logger.Error(err, "Failed to remove Git repository directory", "path", tracker.path)
		}

		r.Eventf(gr, v1.EventTypeNormal, "TrackingStopped", "Stopped tracking")
		r.setStatusCondition(
			ctx, gr,
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "TrackingStopped"},
			metav1.Condition{Type: gitRepositoryClonedCondition, Status: metav1.ConditionFalse, Reason: "TrackingStopped"},
			metav1.Condition{Type: gitRepositoryTrackedCondition, Status: metav1.ConditionFalse, Reason: "TrackingStopped"},
		)
	}
	return nil
}

func (r *GitRepositoryReconciler) loop(uid types.UID) {
	for {
		r.Lock()
		tracker, ok := r.trackers[uid]
		if !ok {
			// Ticker was removed, stop this goroutine
			r.Unlock()
			return
		}

		select {
		case ts, ok := <-tracker.ticker.C:
			if ok {
				r.Unlock()
				r.tick(log.IntoContext(context.Background(), tracker.logger), uid, tracker, ts)
			} else {
				r.Unlock()
				return
			}
		default:
			r.Unlock()
			time.Sleep(5 * time.Second)
		}
	}
}

func (r *GitRepositoryReconciler) tick(ctx context.Context, _ types.UID, tracker *trackingInfo, _ time.Time) {
	r.Lock()
	defer r.Unlock()

	logger := tracker.logger.WithValues("path", tracker.path, "url", tracker.url, "branch", tracker.branch)

	var gr v1alpha1.GitRepository
	if err := r.Get(ctx, types.NamespacedName{Namespace: tracker.namespace, Name: tracker.name}, &gr); err != nil {
		logger.Error(err, "Failed to get tracked GitRepository")
		return
	}

	refName := plumbing.ReferenceName(tracker.branch)
	var repo *git.Repository

	// Clone or open the repository; fail-fast in case either fails
	if _, err := os.Stat(tracker.path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			msg := fmt.Errorf("failed to stat '%s': %w", tracker.path, err).Error()
			r.Eventf(&gr, v1.EventTypeWarning, "CloneOpenFailed", msg)
			r.setStatusCondition(
				ctx, &gr,
				metav1.Condition{Type: gitRepositoryClonedCondition, Status: metav1.ConditionFalse, Reason: "StatFailed", Message: msg},
				metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "InspectCloneFailed", Message: msg},
			)
			return
		}

		b := bytes.Buffer{}
		cloneOptions := git.CloneOptions{URL: tracker.url, ReferenceName: refName, Progress: &b}
		if repository, err := git.PlainClone(tracker.path, false, &cloneOptions); err != nil {
			msg := fmt.Sprintf("Clone failed in '%s': %s\n%s", tracker.path, err, b.String())
			r.Eventf(&gr, v1.EventTypeWarning, "CloneFailed", msg)
			gr.Status.WorkDirectory = ""
			gr.Status.LastPulledSHA = ""
			r.setStatusCondition(
				ctx, &gr,
				metav1.Condition{Type: gitRepositoryClonedCondition, Status: metav1.ConditionFalse, Reason: "CloneFailed", Message: msg},
				metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "CloneFailed", Message: msg},
			)

			if err := os.RemoveAll(tracker.path); err != nil {
				logger.Error(err, "Failed to remove failed clone directory")
			}

			return
		} else {
			r.Eventf(&gr, v1.EventTypeNormal, "CloneSucceeded", "Cloned '%s'\n%s", tracker.url, b.String())
			repo = repository
		}
	} else if repository, err := git.PlainOpen(tracker.path); err != nil {
		msg := fmt.Sprintf("Clone open of '%s' failed: %v", tracker.path, err)
		r.Eventf(&gr, v1.EventTypeWarning, "CloneOpenFailed", msg)
		r.setStatusCondition(ctx, &gr, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "CloneFailed", Message: msg})
		return
	} else {
		repo = repository
	}
	gr.Status.WorkDirectory = tracker.path

	// Obtain the upstream origin (fetch)
	origin, err := repo.Remote("origin")
	if err != nil {
		msg := fmt.Sprintf("Failed to get remote: %s", err)
		r.Eventf(&gr, v1.EventTypeWarning, "RemoteLookupFailed", msg)
		r.setStatusCondition(ctx, &gr, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "RemoteLookupFailed", Message: msg})
		return
	}

	// Fetch remote changes in origin
	b := bytes.Buffer{}
	if err := origin.Fetch(&git.FetchOptions{Progress: &b, Tags: git.AllTags}); err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			msg := fmt.Sprintf("Fetch for remote failed: %s\n%s", err, b.String())
			r.Eventf(&gr, v1.EventTypeWarning, "RemoteFetchFailed", msg)
			r.setStatusCondition(ctx, &gr, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "RemoteFetchFailed", Message: msg})
			return
		} else {
			if b.Len() == 0 {
				b.WriteString("(up to date)")
			}
			msg := fmt.Sprintf("Fetched changes from remote:\n%s", b.String())
			r.Eventf(&gr, v1.EventTypeNormal, "RemoteFetched", msg)
		}
	}

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		msg := fmt.Sprintf("Fetch to get worktree: %s", err)
		r.Eventf(&gr, v1.EventTypeWarning, "WorktreeGetFailed", msg)
		r.setStatusCondition(ctx, &gr, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "WorktreeGetFailed", Message: msg})
		return
	}

	// Ensure the worktree points to the correct reference (branch)
	if err := worktree.Checkout(&git.CheckoutOptions{Branch: refName, Force: true}); err != nil {
		msg := fmt.Sprintf("Checkout of branch '%s' failed: %s", tracker.branch, err)
		r.Eventf(&gr, v1.EventTypeWarning, "CheckoutFailed", msg)
		r.setStatusCondition(ctx, &gr, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "CheckoutFailed", Message: msg})
		return
	}

	// Update the worktree branch
	if err = worktree.Pull(&git.PullOptions{Progress: os.Stdout}); err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			msg := fmt.Sprintf("Pull of branch '%s' failed: %s", tracker.branch, err)
			r.Eventf(&gr, v1.EventTypeWarning, "PullFailed", msg)
			r.setStatusCondition(ctx, &gr, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "PullFailed", Message: msg})
			return
		} else {
			r.Eventf(&gr, v1.EventTypeNormal, "PulledWithNoChanges", "No changes pulled (up-to-date)")
		}
	}

	// Get the current HEAD SHA
	head, err := repo.Head()
	if err != nil {
		msg := fmt.Sprintf("Get HEAD failed: %s", err)
		r.Eventf(&gr, v1.EventTypeWarning, "GetHeadFailed", msg)
		r.setStatusCondition(ctx, &gr, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "GetHeadFailed", Message: msg})
		return
	}

	// Set status
	gr.Status.LastPulledSHA = head.Hash().String()
	r.setStatusCondition(
		ctx, &gr,
		metav1.Condition{Type: gitRepositoryClonedCondition, Status: metav1.ConditionTrue, Reason: "Cloned"},
		metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Cloned"},
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *GitRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.EventRecorder = mgr.GetEventRecorderFor("gitrepository")
	r.Scheme = mgr.GetScheme()
	r.Locker = &sync.Mutex{}
	r.trackers = make(map[types.UID]*trackingInfo)
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GitRepository{}).
		Complete(r)
}

func (r *GitRepositoryReconciler) setStatusCondition(ctx context.Context, gr *v1alpha1.GitRepository, conditions ...metav1.Condition) {
	for _, c := range conditions {
		if c.ObservedGeneration == 0 {
			c.ObservedGeneration = gr.Generation
		}
		if c.LastTransitionTime.IsZero() {
			c.LastTransitionTime = metav1.Time{Time: time.Now()}
		}
		meta.SetStatusCondition(&gr.Status.Conditions, c)
	}
	r.setStatus(ctx, gr)
}

func (r *GitRepositoryReconciler) setStatus(ctx context.Context, gr *v1alpha1.GitRepository) {
	if err := r.Status().Update(ctx, gr); err != nil {
		// TODO: keep retrying if we're just out of date
		r.Eventf(gr, v1.EventTypeWarning, "StatusUpdateFailed", "Failed to update status: %v", err)
	}
}
