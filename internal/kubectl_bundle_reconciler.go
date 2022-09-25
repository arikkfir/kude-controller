package internal

import (
	"bytes"
	"context"
	"fmt"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	kstrings "k8s.io/utils/strings"
	"os/exec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sort"
	"strings"
	"sync"
	"time"
)

type kubectlBundleTracker struct {
	locker    sync.Locker
	logger    logr.Logger
	interval  time.Duration
	ticker    *time.Ticker
	namespace string
	name      string
}

// KubectlBundleReconciler reconciles a KubectlBundle object
type KubectlBundleReconciler struct {
	client.Client
	record.EventRecorder
	Scheme       *runtime.Scheme
	trackersLock sync.Locker                         // Lock for accessing the trackers map
	trackers     map[types.UID]*kubectlBundleTracker // Mapping of tracking info per GitRepository UID
}

//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles/finalizers,verbs=update
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlruns/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;patch

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
			if err := r.stopTracking(ctx, &bundle); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&bundle, kudeFinalizerName)
			if err := r.Update(ctx, &bundle); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if err := r.track(ctx, &bundle); err != nil {
		return ctrl.Result{Requeue: false}, err
	}

	return ctrl.Result{}, nil
}

func (r *KubectlBundleReconciler) track(ctx context.Context, kb *v1alpha1.KubectlBundle) error {
	r.trackersLock.Lock()
	defer r.trackersLock.Unlock()

	pi, err := time.ParseDuration(kb.Spec.DriftDetectionInterval)
	if err != nil {
		tracker, ok := r.trackers[kb.UID]
		if ok {
			tracker.ticker.Stop()
			delete(r.trackers, kb.UID)
		}
		msg := fmt.Sprintf("Failed to parse drift detection interval '%s': %v", kb.Spec.DriftDetectionInterval, err)
		r.Eventf(kb, v1.EventTypeWarning, "InvalidDriftDetectionInterval", msg)
		r.setStatusCondition(
			ctx,
			kb,
			metav1.Condition{Type: kudeTrackedCondition, Status: metav1.ConditionFalse, Reason: "InvalidDriftDetectionInterval", Message: msg},
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "InvalidDriftDetectionInterval", Message: msg},
		)
		return err
	}

	tracker, ok := r.trackers[kb.UID]
	if !ok {
		tracker = &kubectlBundleTracker{
			locker:    &sync.RWMutex{},
			logger:    log.FromContext(ctx).WithName("ticker"),
			interval:  pi,
			ticker:    time.NewTicker(pi),
			namespace: kb.Namespace,
			name:      kb.Name,
		}
		r.trackers[kb.UID] = tracker

		r.Eventf(kb, v1.EventTypeNormal, "TrackingStarted", "Started tracking")
		r.setStatusCondition(
			ctx, kb,
			metav1.Condition{Type: kudeTrackedCondition, Status: metav1.ConditionTrue, Reason: "TrackingStarted"},
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "TrackingStarted"},
		)
		go r.loop(kb.UID)
	} else {
		tracker.locker.Lock()
		defer tracker.locker.Unlock()
		if pi != tracker.interval {
			tracker.ticker.Reset(pi)
		}
	}
	return nil
}

func (r *KubectlBundleReconciler) stopTracking(ctx context.Context, kb *v1alpha1.KubectlBundle) error {
	r.trackersLock.Lock()
	defer r.trackersLock.Unlock()

	tracker, ok := r.trackers[kb.UID]
	if ok {
		tracker.locker.Lock()
		defer tracker.locker.Unlock()

		tracker.ticker.Stop()
		delete(r.trackers, kb.UID)

		r.Eventf(kb, v1.EventTypeNormal, "TrackingStopped", "Stopped tracking")
		r.setStatusCondition(
			ctx, kb,
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "TrackingStopped"},
			metav1.Condition{Type: kudeTrackedCondition, Status: metav1.ConditionFalse, Reason: "TrackingStopped"},
		)
	}
	return nil
}

func (r *KubectlBundleReconciler) loop(uid types.UID) {
	for {
		r.trackersLock.Lock()
		tracker, ok := r.trackers[uid]
		if !ok {
			// Ticker was removed, stop this goroutine
			r.trackersLock.Unlock()
			return
		}
		r.trackersLock.Unlock()

		tracker.locker.Lock()
		select {
		case ts, ok := <-tracker.ticker.C:
			if ok {
				r.tick(log.IntoContext(context.Background(), tracker.logger), uid, tracker, ts)
				tracker.locker.Unlock()
			} else {
				tracker.locker.Unlock()
				return
			}
		default:
			tracker.locker.Unlock()
			time.Sleep(5 * time.Second)
		}
	}
}

func (r *KubectlBundleReconciler) tick(ctx context.Context, _ types.UID, tracker *kubectlBundleTracker, _ time.Time) {

	// Fetch bundle
	var bundle v1alpha1.KubectlBundle
	if err := r.Get(ctx, types.NamespacedName{Namespace: tracker.namespace, Name: tracker.name}, &bundle); err != nil {
		tracker.logger.Error(err, "Failed to get KubectlBundle")
		return
	}

	runs := &v1alpha1.KubectlRunList{}
	if err := r.List(ctx, runs, client.InNamespace(bundle.Namespace), client.MatchingLabels{kudeOwnerUID: string(bundle.UID)}); err != nil {
		msg := fmt.Errorf("could not list previous runs: %w", err).Error()
		r.Eventf(&bundle, v1.EventTypeWarning, "FailedListingRuns", msg)
		r.setStatusCondition(
			ctx, &bundle,
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "FailedListingRuns", Message: msg},
		)
		return
	}
	sort.Stable(sort.Reverse(runs))
	limit := bundle.Spec.RunsHistoryLimit
	if limit <= 0 {
		limit = 10
	}
	if len(runs.Items) > limit {
		oldRuns := runs.Items[limit:]
		for _, run := range oldRuns {
			if err := r.Delete(ctx, &run); err != nil {
				msg := fmt.Errorf("could not delete old run '%s/%s': %w", run.Namespace, run.Name, err).Error()
				r.Eventf(&bundle, v1.EventTypeWarning, "FailedDeletingRun", msg)
				r.Eventf(&run, v1.EventTypeWarning, "FailedDeletingRun", msg)
				return
			}
		}
	}

	// Fetch source GitRepository
	var repo v1alpha1.GitRepository
	gitRepoNamespace, gitRepoName := kstrings.SplitQualifiedName(bundle.Spec.SourceRepository)
	if err := r.Get(ctx, types.NamespacedName{Namespace: gitRepoNamespace, Name: gitRepoName}, &repo); err != nil {
		msg := fmt.Errorf("could not find GitRepository '%s': %w", bundle.Spec.SourceRepository, err).Error()
		r.Eventf(&bundle, v1.EventTypeWarning, "GitRepositoryNotFound", msg)
		r.setStatusCondition(
			ctx, &bundle,
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "GitRepositoryNotFound", Message: msg},
		)
		return
	}

	// Ensure GitRepository is ready to be used
	condition := meta.FindStatusCondition(repo.Status.Conditions, "Ready")
	if condition == nil || condition.Status != metav1.ConditionTrue || repo.Status.LastPulledSHA == "" {
		msg := fmt.Errorf("GitRepository '%s' is not ready", bundle.Spec.SourceRepository).Error()
		r.Eventf(&bundle, v1.EventTypeWarning, "GitRepositoryNotReady", msg)
		r.setStatusCondition(
			ctx, &bundle,
			metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: "GitRepositoryNotReady", Message: msg},
		)
		return
	}

	// Bundle is ready (though that does not ensure successful runs - those are separate "KubectlRun" objects)
	r.setStatusCondition(ctx, &bundle, metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "PreconditionsSatisfied", Message: ""})

	// Create command
	args := make([]string, 0)
	args = append(args, "apply")
	args = append(args, bundle.Spec.Args...)
	args = append(args, "-f")
	args = append(args, bundle.Spec.Files...)
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Dir = repo.Status.WorkDirectory
	run, err := r.createRun(ctx, &bundle, cmd.Dir, cmd.Path, cmd.Args)
	if err != nil {
		msg := fmt.Errorf("could not create KubectlRun: %w", err).Error()
		r.Eventf(&bundle, v1.EventTypeWarning, "FailedCreatingRun", msg)
		return
	}

	// Start the command
	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("$ %s\n", strings.Join(cmd.Args, " ")))
	cmd.Stdout = &b
	cmd.Stderr = &b
	if err := cmd.Start(); err != nil {
		run.Status.ExitCode = -1
		run.Status.Error = fmt.Errorf("failed to start command: %w", err).Error()
		if err := r.Status().Update(ctx, run); err != nil {
			log.FromContext(ctx).Error(err, "Failed to update KubectlRun status")
		}
		return
	}

	// Wait for the command to finish
	err = cmd.Wait()
	run.Status.ExitCode = cmd.ProcessState.ExitCode()
	run.Status.Output = b.String()
	if err != nil {
		run.Status.Error = fmt.Errorf("command failed: %w", err).Error()
	}
	if err := r.Status().Update(ctx, run); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update KubectlRun status")
	}
}

func (r *KubectlBundleReconciler) createRun(ctx context.Context, bundle *v1alpha1.KubectlBundle, dir, command string, args []string) (*v1alpha1.KubectlRun, error) {
	run := v1alpha1.KubectlRun{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				kudeOwnerUID: string(bundle.UID),
			},
			Name:      string(uuid.NewUUID()),
			Namespace: bundle.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				// Ensure this run is deleted when the bundle is deleted
				*metav1.NewControllerRef(bundle, v1alpha1.GroupVersion.WithKind("KubectlBundle")),
			},
		},
		Spec: v1alpha1.KubectlRunSpec{
			Directory: dir,
			Command:   command,
			Args:      args,
		},
	}
	if err := r.Create(ctx, &run); err != nil {
		return nil, fmt.Errorf("failed to create a bundle run: %w", err)
	}
	return &run, nil
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
	r.EventRecorder = mgr.GetEventRecorderFor("kubectlbundle")
	r.trackersLock = &sync.RWMutex{}
	r.trackers = make(map[types.UID]*kubectlBundleTracker)

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

func (r *KubectlBundleReconciler) setStatusCondition(ctx context.Context, kb *v1alpha1.KubectlBundle, conditions ...metav1.Condition) {
	for _, c := range conditions {
		if c.ObservedGeneration == 0 {
			c.ObservedGeneration = kb.Generation
		}
		if c.LastTransitionTime.IsZero() {
			c.LastTransitionTime = metav1.Time{Time: time.Now()}
		}
		meta.SetStatusCondition(&kb.Status.Conditions, c)
	}
	r.setStatus(ctx, kb)
}

func (r *KubectlBundleReconciler) setStatus(ctx context.Context, o client.Object) {
	if err := r.Status().Update(ctx, o); err != nil {
		// TODO: keep retrying if we're just out of date
		r.Eventf(o, v1.EventTypeWarning, "StatusUpdateFailed", "Failed to update status: %v", err)
	}
}
