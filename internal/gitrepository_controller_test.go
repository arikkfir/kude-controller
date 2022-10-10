package internal

import (
	"context"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	"os"
	"os/exec"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
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

func TestIgnoreMissingResource(t *testing.T) {
	reconciler := &GitRepositoryReconciler{}
	_, _ = setupTestEnv(t, reconciler)

	time.Sleep(5 * time.Second) // Give manager and cache time to start
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
	k8sClient, _ := setupTestEnv(t, &GitRepositoryReconciler{WorkDir: "/tmp"})

	repo := &v1alpha1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.GitRepository{}).Name(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo1",
			Namespace: "default",
		},
		Spec: v1alpha1.GitRepositorySpec{},
	}
	lookupKey := types.NamespacedName{Name: repo.Name, Namespace: repo.Namespace}

	ctx := context.Background()
	assert.NoErrorf(t, k8sClient.Create(ctx, repo), "resource creation failed")
	assert.Eventuallyf(t, func() bool {
		var r v1alpha1.GitRepository
		return assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") &&
			meta.IsStatusConditionFalse(r.Status.Conditions, typeAvailableGitRepository)
	}, 30*time.Second, time.Second, "expected condition '%s' to become False", typeAvailableGitRepository)
	assert.Eventuallyf(t, func() bool {
		var r v1alpha1.GitRepository
		return assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") &&
			meta.IsStatusConditionPresentAndEqual(r.Status.Conditions, typeClonedGitRepository, metav1.ConditionUnknown)
	}, 30*time.Second, time.Second, "expected condition '%s' to become Unknown", typeClonedGitRepository)
	assert.Eventuallyf(t, func() bool {
		var r v1alpha1.GitRepository
		return assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") &&
			slices.Contains(r.Finalizers, finalizerGitRepository)
	}, 30*time.Second, time.Second, "expected finalizer '%s' to be added", finalizerGitRepository)
	assert.Eventuallyf(t, func() bool {
		var r v1alpha1.GitRepository
		return assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") &&
			strings.HasPrefix(r.Status.WorkDirectory, "/tmp/")
	}, 30*time.Second, time.Second, "expected work directory to have '/tmp' prefix")
}

func TestGitRepositoryClone(t *testing.T) {
	if !hasGit {
		t.Skip("git not found, skipping")
	}
	repository, err := newGitRepository(t.Name())
	require.NoErrorf(t, err, "failed to create repository")
	require.NoErrorf(t, repository.commitFile("file1", "content1"), "failed to commit file")
	defer os.RemoveAll(repository.dir)

	k8sClient, _ := setupTestEnv(t, &GitRepositoryReconciler{WorkDir: "/tmp"})

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
			URL:             repository.url.String(),
			Branch:          "refs/heads/main",
			PollingInterval: "5s",
		},
	}
	lookupKey := types.NamespacedName{Name: repo.Name, Namespace: repo.Namespace}

	ctx := context.Background()
	assert.NoErrorf(t, k8sClient.Create(ctx, repo), "resource creation failed")
	timeout := 10 * time.Second
	assert.Eventuallyf(t, func() bool {
		var r v1alpha1.GitRepository
		return assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") &&
			meta.IsStatusConditionTrue(r.Status.Conditions, typeAvailableGitRepository)
	}, timeout, time.Second, "expected condition '%s' to become True", typeAvailableGitRepository)
	assert.Eventuallyf(t, func() bool {
		var r v1alpha1.GitRepository
		return assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") &&
			meta.IsStatusConditionTrue(r.Status.Conditions, typeClonedGitRepository)
	}, timeout, time.Second, "expected condition '%s' to become True", typeClonedGitRepository)
	assert.Eventuallyf(t, func() bool {
		var r v1alpha1.GitRepository
		return assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") &&
			slices.Contains(r.Finalizers, finalizerGitRepository)
	}, timeout, time.Second, "expected finalizer '%s' to be added", finalizerGitRepository)
	assert.Eventuallyf(t, func() bool {
		var r v1alpha1.GitRepository
		return assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") &&
			strings.HasPrefix(r.Status.WorkDirectory, "/tmp/")
	}, timeout, time.Second, "expected work directory to have '/tmp' prefix")

	// TODO: verify clone dir
}
