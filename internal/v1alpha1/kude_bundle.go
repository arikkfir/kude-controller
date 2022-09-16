package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KudeBundleSpec describes the desired state of a KudeBundle in the cluster. It provides the necessary
// information on the manifests to be installed in the cluster.
type KudeBundleSpec struct {
	Files []string `json:"files,omitempty"`

	// +kubebuilder:validation:Pattern=`^[^/]+/[^/]+$`
	SourceRepository string `json:"sourceRepository,omitempty"`
}

// KudeBundleStatus defines the observed state of a KudeBundle.
type KudeBundleStatus struct {
	Errors []string `json:"errors,omitempty"` // List of errors encountered while applying the files
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KudeBundle defines a set of Kubernetes manifest YAML files to be applied in the cluster.
type KudeBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KudeBundleSpec   `json:"spec,omitempty"`
	Status KudeBundleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KudeBundleList contains a list of KudeBundle
type KudeBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KudeBundle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KudeBundle{}, &KudeBundleList{})
}
