package object

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Status interface {
	GetConditions() *[]metav1.Condition
}
