package internal

import (
	"context"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	"github.com/arikkfir/kude-controller/test/gittest"
	"github.com/arikkfir/kude-controller/test/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"os/exec"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

var (
	hasGit = false
)

func init() {
	if _, err := exec.LookPath("git"); err == nil {
		hasGit = true
	}
}

func TestIgnoreMissingGitRepositoryResource(t *testing.T) {
	reconciler := &GitRepositoryReconciler{}
	_, _ = harness.SetupTestEnv(t, reconciler)

	time.Sleep(5 * time.Second) // Give manager and cache time to start; needed since we're directly invoking controller
	res, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "ns1",
			Name:      "r1",
		},
	})
	assert.NoErrorf(t, err, "expected reconciliation for missing resource NOT to fail")
	assert.Falsef(t, res.Requeue, "expected reconciliation for missing resource NOT to request requeuing, got: %+v", res)
}

func TestGitRepositoryResourceInitialization(t *testing.T) {
	k8sClient, _ := harness.SetupTestEnv(t, &GitRepositoryReconciler{WorkDir: "/tmp"})

	repo := &v1alpha1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.GitRepository{}).Name(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo1",
			Namespace: "default",
		},
		Spec: v1alpha1.GitRepositorySpec{
			PollingInterval: "10s",
		},
	}
	lookupKey := types.NamespacedName{Name: repo.Name, Namespace: repo.Namespace}

	ctx := context.Background()
	require.NoErrorf(t, k8sClient.Create(ctx, repo), "resource creation failed")
	assert.EventuallyWithTf(t, func(c *assert.CollectT) {
		var r v1alpha1.GitRepository
		if assert.NoErrorf(c, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") {
			assert.Contains(c, r.Finalizers, finalizerGitRepository, "finalizer not found")
			assert.Equal(c, string("/tmp/"+r.UID), r.Status.WorkDirectory, "work directory not set correctly")

			cCloned := meta.FindStatusCondition(r.Status.Conditions, typeClonedGitRepository)
			assert.Equal(c, metav1.ConditionFalse, cCloned.Status, "incorrect status")
			assert.Equal(c, "NotCloned", cCloned.Reason, "incorrect reason")
			assert.Equal(c, "", cCloned.Message, "incorrect message")

			cAvailable := meta.FindStatusCondition(r.Status.Conditions, typeAvailableGitRepository)
			assert.Equal(c, metav1.ConditionFalse, cAvailable.Status, "incorrect status")
			assert.Equal(c, "NotCloned", cAvailable.Reason, "incorrect reason")
			assert.Equal(c, "", cAvailable.Message, "incorrect message")
		}
	}, 5*time.Second, 1*time.Second, "resource not initialized correctly")
}

func TestGitRepositoryClone(t *testing.T) {
	if !hasGit {
		t.Skip("git not found, skipping")
	}
	repository, err := gittest.NewGitRepository(t.Name())
	require.NoErrorf(t, err, "failed to create repository")
	require.NoErrorf(t, repository.CommitFile("file1", "content1"), "failed to commit file")
	defer os.RemoveAll(repository.Dir)

	k8sClient, _ := harness.SetupTestEnv(t, &GitRepositoryReconciler{WorkDir: "/tmp"})

	repo := &v1alpha1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.GitRepository{}).Name(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo1",
			Namespace: "default",
		},
		Spec: v1alpha1.GitRepositorySpec{
			URL:             repository.URL.String(),
			Branch:          "refs/heads/main",
			PollingInterval: "5s",
		},
	}
	lookupKey := types.NamespacedName{Name: repo.Name, Namespace: repo.Namespace}

	ctx := context.Background()
	assert.NoErrorf(t, k8sClient.Create(ctx, repo), "resource creation failed")
	assert.EventuallyWithTf(t, func(c *assert.CollectT) {
		var r v1alpha1.GitRepository
		if assert.NoErrorf(c, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") {
			cCloned := meta.FindStatusCondition(r.Status.Conditions, typeClonedGitRepository)
			assert.Equal(c, metav1.ConditionTrue, cCloned.Status, "incorrect status")
			assert.Equal(c, "Cloned", cCloned.Reason, "incorrect reason")
			assert.Equal(c, "", cCloned.Message, "incorrect message")

			cAvailable := meta.FindStatusCondition(r.Status.Conditions, typeAvailableGitRepository)
			assert.Equal(c, metav1.ConditionTrue, cAvailable.Status, "incorrect status")
			assert.Equal(c, "Ready", cAvailable.Reason, "incorrect reason")
			assert.Equal(c, "", cAvailable.Message, "incorrect message")
		}
	}, 5*time.Second, 1*time.Second, "resource not cloned correctly")

	// TODO: verify clone dir
}

func TestGitRepositoryDeletion(t *testing.T) {
	k8sClient, _ := harness.SetupTestEnv(t, &KubectlBundleReconciler{})

	repo := &v1alpha1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.GitRepository{}).Name(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo1",
			Namespace: "default",
			Finalizers: []string{
				"Tests",
			},
		},
		Spec: v1alpha1.GitRepositorySpec{
			Branch:          "refs/heads/main",
			PollingInterval: "5s",
		},
	}
	lookupKey := types.NamespacedName{Name: repo.Name, Namespace: repo.Namespace}

	ctx := context.Background()
	require.NoErrorf(t, k8sClient.Create(ctx, repo), "resource creation failed")
	time.Sleep(3 * time.Second)
	require.NoErrorf(t, k8sClient.Delete(ctx, repo), "resource deletion failed")
	assert.EventuallyWithTf(t, func(c *assert.CollectT) {
		var r v1alpha1.GitRepository
		if assert.NoErrorf(c, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") {

			cDegraded := meta.FindStatusCondition(r.Status.Conditions, typeDegradedGitRepository)
			assert.Equal(c, metav1.ConditionTrue, cDegraded.Status, "incorrect status")
			assert.Equal(c, "Deleted", cDegraded.Reason, "incorrect reason")
			assert.Equal(c, "Deleting resource", cDegraded.Message, "incorrect message")

			cUpToDate := meta.FindStatusCondition(r.Status.Conditions, typeAvailableGitRepository)
			assert.Equal(c, metav1.ConditionFalse, cUpToDate.Status, "incorrect status")
			assert.Equal(c, "Deleted", cUpToDate.Reason, "incorrect reason")
			assert.Equal(c, "Deleting resource", cUpToDate.Message, "incorrect message")

			cCloned := meta.FindStatusCondition(r.Status.Conditions, typeClonedGitRepository)
			assert.Equal(c, metav1.ConditionFalse, cCloned.Status, "incorrect status")
			assert.Equal(c, "Deleted", cCloned.Reason, "incorrect reason")
			assert.Equal(c, "", cCloned.Message, "incorrect message")

			assert.Equal(c, "", r.Status.WorkDirectory)

			assert.NotContains(c, r.Finalizers, finalizerGitRepository, "finalizer found")
		}
	}, 15*time.Second, 1*time.Second, "resource not finalized correctly")

	if assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, repo), "resource lookup failed") {
		assert.Equal(t, []string{"Tests"}, repo.Finalizers)
		repo.ObjectMeta.Finalizers = []string{}
		require.NoErrorf(t, k8sClient.Update(ctx, repo), "finalizer removal failed")
		assert.EventuallyWithTf(t, func(c *assert.CollectT) {
			var r v1alpha1.GitRepository
			assert.NoError(c, client.IgnoreNotFound(k8sClient.Get(ctx, lookupKey, &r)))
		}, 5*time.Second, 1*time.Second, "resource not finalized correctly")
	}
}
