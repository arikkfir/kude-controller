package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KustomizeBundleSpec describes the desired state of a KustomizeBundle in the cluster. It provides the necessary
// information on the manifests to be installed in the cluster.
type KustomizeBundleSpec struct {
	Files []string `json:"files,omitempty"`

	// +kubebuilder:validation:Pattern=`^[^/]+/[^/]+$`
	SourceRepository string `json:"sourceRepository,omitempty"`
}

// KustomizeBundleStatus defines the observed state of a KustomizeBundle.
type KustomizeBundleStatus struct {
	Errors []string `json:"errors,omitempty"` // List of errors encountered while applying the files
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KustomizeBundle defines a set of Kubernetes manifest YAML files to be applied in the cluster.
type KustomizeBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KustomizeBundleSpec   `json:"spec,omitempty"`
	Status KustomizeBundleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KustomizeBundleList contains a list of KustomizeBundle
type KustomizeBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KustomizeBundle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KustomizeBundle{}, &KustomizeBundleList{})
}
