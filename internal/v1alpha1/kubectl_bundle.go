package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubectlBundleSpec describes the desired state of a KubectlBundle in the cluster. It provides the necessary
// information on the manifests to be installed in the cluster.
type KubectlBundleSpec struct {
	Args []string `json:"args,omitempty"` // Arguments to pass to the kubectl command

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Files []string `json:"files,omitempty"` // Files to apply

	// +kubebuilder:validation:Pattern=`^[^/]+/[^/]+$`
	SourceRepository string `json:"sourceRepository,omitempty"` // Source repository to pull the files from

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DriftDetectionInterval string `json:"driftDetectionInterval,omitempty"` // Drift verification interval

	// +kubebuilder:validation:Minimum=1
	RunsHistoryLimit int `json:"runsHistoryLimit,omitempty"` // Runs history limit
}

// KubectlBundleStatus defines the observed state of a KubectlBundle.
type KubectlBundleStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KubectlBundle defines a set of Kubernetes manifest YAML files to be applied in the cluster.
type KubectlBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubectlBundleSpec   `json:"spec,omitempty"`
	Status KubectlBundleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubectlBundleList contains a list of KubectlBundle
type KubectlBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubectlBundle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubectlBundle{}, &KubectlBundleList{})
}
