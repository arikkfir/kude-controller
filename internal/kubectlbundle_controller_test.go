package internal

import (
	"context"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	"github.com/arikkfir/kude-controller/test/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"testing"
	"time"
)

func TestKubectlBundleInitialization(t *testing.T) {
	k8sClient, _ := harness.SetupTestEnv(t, &KubectlBundleReconciler{})

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
