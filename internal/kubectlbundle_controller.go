package internal

import (
	"bytes"
	"context"
	"fmt"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sort"
	"strings"
	"time"
)

const (
	finalizerKubectlBundle    = "kubectlbundles.kude.kfirs.com/finalizer"
	typeUpToDateKubectlBundle = "UpToDate"                               // Is the ®KubectlBundle up to date?
	typeDegradedKubectlBundle = "Degraded"                               // When the KubectlBundle is deleted, but finalizer not applied yet
	ownerUIDKubectlBundle     = "kubectlbundles.kude.kfirs.com/ownerUID" // Label for setting the owner UID
)

// KubectlBundleReconciler reconciles a KubectlBundle object
type KubectlBundleReconciler struct {
	Client   client.Client        // Kubernetes API client
	Recorder record.EventRecorder // Kubernetes event recorder
	Scheme   *runtime.Scheme      // Scheme registry
}

//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=kubectlbundles/finalizers,verbs=update
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=commandruns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kude.kfirs.com,resources=commandruns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile continuously aims to move the current state of [KubectlBundle] objects closer to their desired state.
func (r *KubectlBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var o v1alpha1.KubectlBundle
	if err := r.Client.Get(ctx, req.NamespacedName, &o); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Ensure statuses have the "Unknown" value when they are missing
	if meta.FindStatusCondition(o.Status.Conditions, typeUpToDateKubectlBundle) == nil {
		if res, err := r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionUnknown, "Reconciling", "Initial value"); res.Requeue || err != nil {
			return res, err
		} else {
			return ctrl.Result{Requeue: true}, nil
		}
	}
	if meta.FindStatusCondition(o.Status.Conditions, typeDegradedKubectlBundle) == nil {
		if res, err := r.setCondition(ctx, &o, typeDegradedKubectlBundle, metav1.ConditionFalse, "Reconciling", "Initial value"); res.Requeue || err != nil {
			return res, err
		} else {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Add our finalizer
	if controllerutil.AddFinalizer(&o, finalizerKubectlBundle) {
		if err := r.Client.Update(ctx, &o); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update KubectlBundle with finalizer: %w", err)
		} else {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// If marked for deletion, perform actual deletion & remove finalizer
	if o.DeletionTimestamp != nil {
		if res, err := r.setCondition(ctx, &o, typeDegradedKubectlBundle, metav1.ConditionTrue, "Deleted", "Deleting resource"); res.Requeue || err != nil {
			return res, err
		}
		if res, err := r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionUnknown, "Deleted", "Deleting resource"); res.Requeue || err != nil {
			return res, err
		}
		// TODO: consider pruning last run's created objects
		if controllerutil.RemoveFinalizer(&o, finalizerKubectlBundle) {
			if err := r.Client.Update(ctx, &o); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Get interval
	interval, err := time.ParseDuration(o.Spec.DriftDetectionInterval)
	if err != nil {
		if res, err := r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionUnknown, "InvalidPollingInterval", "Invalid polling interval: "+o.Spec.DriftDetectionInterval); res.Requeue || err != nil {
			return res, err
		}
		return ctrl.Result{Requeue: false}, nil
	}

	// Fetch list of runs for this bundle
	runs := &v1alpha1.CommandRunList{}
	if err := r.Client.List(ctx, runs, client.InNamespace(o.Namespace), client.MatchingLabels{ownerUIDKubectlBundle: string(o.UID)}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list command runs: %w", err)
	}

	// Sort by creation time (DESC) and get the latest <limit> runs
	sort.Stable(sort.Reverse(runs))
	limit := o.Spec.RunsHistoryLimit
	if limit <= 0 {
		limit = 10
	}

	// Delete all runs that are over the limit
	var lastRun *v1alpha1.CommandRun = nil
	if len(runs.Items) > 0 {
		lastRun = &runs.Items[0]
		if len(runs.Items) > limit {
			for _, run := range runs.Items[limit:] {
				if err := r.Client.Delete(ctx, &run); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to delete command run '%s/%s': %w", run.Namespace, run.Name, err)
				}
			}
		}
	}

	// Fetch source GitRepository
	var repo v1alpha1.GitRepository
	gitRepoNamespace, gitRepoName := kstrings.SplitQualifiedName(o.Spec.SourceRepository)
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: gitRepoNamespace, Name: gitRepoName}, &repo); err != nil {
		return r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionUnknown, "GitRepositoryNotFound", err.Error())
	}

	// Ensure GitRepository is ready to be used
	if !meta.IsStatusConditionTrue(repo.Status.Conditions, typeAvailableGitRepository) {
		return r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionUnknown, "GitRepositoryNotAvailable", "")
	}

	// Compare SHA of last run to GitRepository SHAl update the UpToDate condition accordingly
	// We're up-to-date if:
	//		- at least one run exists
	//		- last run matches the latest repository commit SHA
	//		- last run was successful
	if lastRun != nil {
		if lastRun.Spec.CommitSHA == repo.Status.LastPulledSHA {
			if lastRun.Status.ExitCode == 0 {
				if res, err := r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionTrue, "UpToDate", "Last run matches current repository SHA"); err != nil || res.Requeue {
					return res, err
				} else {
					return ctrl.Result{RequeueAfter: interval}, nil
				}
			} else if res, err := r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionFalse, "Failed", "Last run failed, retrying"); err != nil || res.Requeue {
				return res, err
			}
		} else if res, err := r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionFalse, "OutOfDate", "Last run does not match current repository SHA"); err != nil || res.Requeue {
			return res, err
		}
	} else if res, err := r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionFalse, "NotApplied", "Bundle has no runs yet"); err != nil || res.Requeue {
		return res, err
	}

	args := make([]string, 0)
	args = append(args, "apply")
	args = append(args, o.Spec.Args...)
	args = append(args, "-f")
	args = append(args, o.Spec.Files...)
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Dir = repo.Status.WorkDirectory
	run, err := r.createRun(ctx, &o, repo.Status.LastPulledSHA, cmd.Dir, cmd.Path, cmd.Args)
	if err != nil {
		r.Recorder.Eventf(&o, v1.EventTypeWarning, "FailedCreatingRun", err.Error())
		return ctrl.Result{RequeueAfter: interval}, err
	}

	// Start the command
	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("$ %s\n", strings.Join(cmd.Args, " ")))
	cmd.Stdout = &b
	cmd.Stderr = &b
	if err := cmd.Start(); err != nil {
		r.Recorder.Eventf(&o, v1.EventTypeWarning, "FailedStartingRun", "Failed to start run '%s': %s\n%s", run.Name, err.Error(), b.String())
		run.Status.ExitCode = -1
		run.Status.Error = fmt.Errorf("failed to start command: %w", err).Error()
		return ctrl.Result{RequeueAfter: interval}, r.Client.Status().Update(ctx, run)
	}

	// Wait for the command to finish, then update status
	if err := cmd.Wait(); err != nil {
		r.Recorder.Eventf(&o, v1.EventTypeWarning, "RunFailed", "Run '%s' failed: %s\n%s", run.Name, err.Error(), b.String())
		run.Status.ExitCode = cmd.ProcessState.ExitCode()
		run.Status.Output = b.String()
		run.Status.Error = fmt.Errorf("command failed: %w", err).Error()
		return ctrl.Result{RequeueAfter: interval}, r.Client.Status().Update(ctx, run)
	} else {
		run.Status.ExitCode = cmd.ProcessState.ExitCode()
		run.Status.Output = b.String()
		if err := r.Client.Status().Update(ctx, run); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update CommandRun status: %w", err)
		}
		if res, err := r.setCondition(ctx, &o, typeUpToDateKubectlBundle, metav1.ConditionTrue, "UpToDate", "Successful run"); err != nil {
			return res, err
		} else {
			return ctrl.Result{RequeueAfter: interval}, nil
		}
	}
}

func (r *KubectlBundleReconciler) createRun(ctx context.Context, bundle *v1alpha1.KubectlBundle, commitSHA, dir, command string, args []string) (*v1alpha1.CommandRun, error) {
	run := v1alpha1.CommandRun{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				ownerUIDKubectlBundle: string(bundle.UID),
			},
			Name:      string(uuid.NewUUID()),
			Namespace: bundle.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				// Ensure this run is deleted when the bundle is deleted
				*metav1.NewControllerRef(bundle, v1alpha1.GroupVersion.WithKind("KubectlBundle")),
			},
		},
		Spec: v1alpha1.CommandRunSpec{
			CommitSHA: commitSHA,
			Directory: dir,
			Command:   command,
			Args:      args,
		},
	}
	if err := r.Client.Create(ctx, &run); err != nil {
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
	err := r.Client.List(context.TODO(), bundles, listOps)
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

func (r *KubectlBundleReconciler) setCondition(ctx context.Context, o *v1alpha1.KubectlBundle, conditionType string, status metav1.ConditionStatus, reason, message string) (ctrl.Result, error) {
	if c := meta.FindStatusCondition(o.Status.Conditions, conditionType); c == nil {
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{
			Type:    conditionType,
			Status:  status,
			Reason:  reason,
			Message: message,
		})
		if err := r.Client.Status().Update(ctx, o); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set status '%s' to '%s' with reason '%s': %w", conditionType, status, reason, err)
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
			return ctrl.Result{}, fmt.Errorf("failed to set status '%s' to '%s' with reason '%s': %w", conditionType, status, reason, err)
		} else {
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubectlBundleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.Recorder = mgr.GetEventRecorderFor("kubectlbundle")

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
