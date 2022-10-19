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

	// Conditions represent the latest available observations of the resource
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KustomizeBundle defines a set of Kubernetes manifest YAML files to be applied in the cluster.
//go:generate go run ../../scripts/objecter/objecter.go -type=KustomizeBundle
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
