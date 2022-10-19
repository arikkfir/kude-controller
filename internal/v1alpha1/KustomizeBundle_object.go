package v1alpha1

import (
	"github.com/arikkfir/kude-controller/internal/object"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (in *KustomizeBundle) GetStatus() object.Status {
	return &in.Status
}

func (in *KustomizeBundleStatus) GetConditions() *[]metav1.Condition {
	return &in.Conditions
}

func (in *KustomizeBundleList) Len() int {
	return len(in.Items)
}

func (in *KustomizeBundleList) Less(i, j int) bool {
	ii := in.Items[i]
	sj := in.Items[j]
	return ii.CreationTimestamp.Before(&sj.CreationTimestamp)
}

func (in *KustomizeBundleList) Swap(i, j int) {
	ii := in.Items[i]
	sj := in.Items[j]
	in.Items[i] = sj
	in.Items[j] = ii
}
