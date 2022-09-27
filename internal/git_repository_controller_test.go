package internal

import (
	"context"
	"github.com/arikkfir/kude-controller/internal/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
)

var _ = Describe("GitRepository controller", func() {

	const (
		GitRepositoryName       = "repo1"
		GitRepositoryNamespace  = "default"
		GitRepositoryMainBranch = "main"
	)

	// TODO: create/tear-down Git repository

	Context("When creating a GitRepository", func() {
		It("Should add a finalizer to the GitRepository", func() {
			ctx := context.Background()

			By("By creating a new GitRepository")
			repo := &v1alpha1.GitRepository{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.GroupVersion.String(),
					Kind:       reflect.TypeOf(v1alpha1.GitRepository{}).Name(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GitRepositoryName,
					Namespace: GitRepositoryNamespace,
				},
				Spec: v1alpha1.GitRepositorySpec{
					URL:             "file:///tmp/arikkfir/kude-controller/testdata/repo1",
					Branch:          GitRepositoryMainBranch,
					PollingInterval: "5s",
				},
			}
			Expect(k8sClient.Create(ctx, repo)).Should(Succeed())

			By("By verifying correct finalizer was added")
			repoLookupKey := types.NamespacedName{Name: GitRepositoryName, Namespace: GitRepositoryNamespace}
			createdRepo := v1alpha1.GitRepository{}
			Eventually(func() ([]string, error) {
				if err := k8sClient.Get(ctx, repoLookupKey, &createdRepo); err != nil {
					return nil, err
				}
				return createdRepo.Finalizers, nil
			}, "10s", "1s").Should(ContainElement(kudeFinalizerName))
		})
	})

})
