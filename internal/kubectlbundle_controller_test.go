package internal

import (
	"context"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	"github.com/arikkfir/kude-controller/test/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

func TestIgnoreMissingKubectlBundleResource(t *testing.T) {
	reconciler := &KubectlBundleReconciler{}
	_, _, _ = harness.SetupTestEnv(t, reconciler)

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

func TestKubectlBundleInitialization(t *testing.T) {
	k8sClient, _, _ := harness.SetupTestEnv(t, &KubectlBundleReconciler{})

	bundle := &v1alpha1.KubectlBundle{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.KubectlBundle{}).Name(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bundle1",
			Namespace: "default",
		},
		Spec: v1alpha1.KubectlBundleSpec{
			DriftDetectionInterval: "5sec",
			SourceRepository:       "ns1/repo1",
			Files:                  []string{"*.yaml"},
		},
	}
	lookupKey := types.NamespacedName{Name: bundle.Name, Namespace: bundle.Namespace}

	ctx := context.Background()
	require.NoErrorf(t, k8sClient.Create(ctx, bundle), "resource creation failed")
	assert.EventuallyWithTf(t, func(c *assert.CollectT) {
		var r v1alpha1.KubectlBundle
		if assert.NoErrorf(c, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") {
			assert.Contains(c, r.Finalizers, finalizerKubectlBundle, "finalizer not found")
		}
	}, 5*time.Second, 1*time.Second, "resource not cloned correctly")
}

func TestKubectlBundleDeletion(t *testing.T) {
	k8sClient, _, _ := harness.SetupTestEnv(t, &KubectlBundleReconciler{})

	bundle := &v1alpha1.KubectlBundle{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       reflect.TypeOf(v1alpha1.KubectlBundle{}).Name(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bundle1",
			Namespace: "default",
			Finalizers: []string{
				"Tests",
			},
		},
		Spec: v1alpha1.KubectlBundleSpec{
			DriftDetectionInterval: "5sec",
			SourceRepository:       "ns1/repo1",
			Files:                  []string{"*.yaml"},
		},
	}
	lookupKey := types.NamespacedName{Name: bundle.Name, Namespace: bundle.Namespace}

	ctx := context.Background()
	require.NoErrorf(t, k8sClient.Create(ctx, bundle), "resource creation failed")
	time.Sleep(3 * time.Second)
	require.NoErrorf(t, k8sClient.Delete(ctx, bundle), "resource deletion failed")
	assert.EventuallyWithTf(t, func(c *assert.CollectT) {
		var r v1alpha1.KubectlBundle
		if assert.NoErrorf(c, k8sClient.Get(ctx, lookupKey, &r), "resource lookup failed") {

			cDegraded := meta.FindStatusCondition(r.Status.Conditions, typeDegradedKubectlBundle)
			if assert.NotNil(c, cDegraded, "degraded condition not found") {
				assert.Equal(c, metav1.ConditionTrue, cDegraded.Status, "incorrect status")
				assert.Equal(c, "Deleted", cDegraded.Reason, "incorrect reason")
				assert.Equal(c, "Deleting resource", cDegraded.Message, "incorrect message")
			}

			cUpToDate := meta.FindStatusCondition(r.Status.Conditions, typeUpToDateKubectlBundle)
			if assert.NotNil(c, cUpToDate, "uptodate condition not found") {
				assert.Equal(c, metav1.ConditionUnknown, cUpToDate.Status, "incorrect status")
				assert.Equal(c, "Deleted", cUpToDate.Reason, "incorrect reason")
				assert.Equal(c, "Deleting resource", cUpToDate.Message, "incorrect message")
			}

			assert.NotContains(c, r.Finalizers, finalizerKubectlBundle, "finalizer found")
		}
	}, 5*time.Second, 1*time.Second, "resource not finalized correctly")

	if assert.NoErrorf(t, k8sClient.Get(ctx, lookupKey, bundle), "resource lookup failed") {
		assert.Equal(t, []string{"Tests"}, bundle.Finalizers)
		bundle.ObjectMeta.Finalizers = []string{}
		require.NoErrorf(t, k8sClient.Update(ctx, bundle), "finalizer removal failed")
		assert.EventuallyWithTf(t, func(c *assert.CollectT) {
			var r v1alpha1.KubectlBundle
			assert.NoError(c, client.IgnoreNotFound(k8sClient.Get(ctx, lookupKey, &r)))
		}, 5*time.Second, 1*time.Second, "resource not finalized correctly")
	}
}
