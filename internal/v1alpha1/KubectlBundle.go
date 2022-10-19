package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubectlBundleSpec describes the desired state of a KubectlBundle in the cluster. It provides the necessary
// information on the manifests to be installed in the cluster.
type KubectlBundleSpec struct {
	// Arguments to pass to the kubectl command
	Args []string `json:"args,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// Files to apply
	Files []string `json:"files"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[^/]+/[^/]+$`
	// Source repository to pull the files from
	SourceRepository string `json:"sourceRepository"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// Drift verification interval
	DriftDetectionInterval string `json:"driftDetectionInterval"`

	// +kubebuilder:validation:Minimum=1
	// Runs history limit
	RunsHistoryLimit int `json:"runsHistoryLimit,omitempty"`
}

// KubectlBundleStatus defines the observed state of a KubectlBundle.
type KubectlBundleStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Files",type="string",JSONPath=".spec.files"
//+kubebuilder:printcolumn:name="Repository",type="string",JSONPath=".spec.sourceRepository"
//+kubebuilder:printcolumn:name="Interval",type="string",JSONPath=".spec.driftDetectionInterval"
//+kubebuilder:printcolumn:name="History limit",type="string",JSONPath=".spec.runsHistoryLimit"

// KubectlBundle defines a set of Kubernetes manifest YAML files to be applied in the cluster.
//go:generate go run ../../scripts/objecter/objecter.go -type=KubectlBundle
type KubectlBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubectlBundleSpec   `json:"spec"`
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
