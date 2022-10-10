package internal

import (
	"context"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	"reflect"
	"testing"
	"time"
)

func TestCreateKubectlBundle(t *testing.T) {
	k8sClient, _ := setupTestEnv(t, &KubectlBundleReconciler{})

	typeName := reflect.TypeOf(v1alpha1.KubectlBundle{}).Name()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	repo := &v1alpha1.KubectlBundle{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       typeName,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repo1",
			Namespace: "default",
		},
		Spec: v1alpha1.KubectlBundleSpec{},
	}
	if err := k8sClient.Create(ctx, repo); err != nil {
		t.Errorf("failed to create %s: %v", typeName, err)
	}

	repoLookupKey := types.NamespacedName{Name: repo.Name, Namespace: repo.Namespace}
	createdObject := v1alpha1.KubectlBundle{}
	for {
		if err := k8sClient.Get(ctx, repoLookupKey, &createdObject); err != nil {
			t.Errorf("failed to get %s: %v", typeName, err)
		} else if slices.Contains(createdObject.Finalizers, finalizerKubectlBundle) {
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}
}
