package v1alpha1

import (
	"github.com/arikkfir/kude-controller/internal/object"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (in *KubectlBundle) GetStatus() object.Status {
	return &in.Status
}

func (in *KubectlBundleStatus) GetConditions() *[]metav1.Condition {
	return &in.Conditions
}

func (in *KubectlBundleList) Len() int {
	return len(in.Items)
}

func (in *KubectlBundleList) Less(i, j int) bool {
	ii := in.Items[i]
	sj := in.Items[j]
	return ii.CreationTimestamp.Before(&sj.CreationTimestamp)
}

func (in *KubectlBundleList) Swap(i, j int) {
	ii := in.Items[i]
	sj := in.Items[j]
	in.Items[i] = sj
	in.Items[j] = ii
}
