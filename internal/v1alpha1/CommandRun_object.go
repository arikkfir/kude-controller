package v1alpha1

import (
	"github.com/arikkfir/kude-controller/internal/object"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (in *CommandRun) GetStatus() object.Status {
	return &in.Status
}

func (in *CommandRunStatus) GetConditions() *[]metav1.Condition {
	return &in.Conditions
}

func (in *CommandRunList) Len() int {
	return len(in.Items)
}

func (in *CommandRunList) Less(i, j int) bool {
	ii := in.Items[i]
	sj := in.Items[j]
	return ii.CreationTimestamp.Before(&sj.CreationTimestamp)
}

func (in *CommandRunList) Swap(i, j int) {
	ii := in.Items[i]
	sj := in.Items[j]
	in.Items[i] = sj
	in.Items[j] = ii
}
