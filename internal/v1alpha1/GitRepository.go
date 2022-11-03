package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitRepositorySpec is the desired state of a monitored Git repository.
type GitRepositorySpec struct {
	// +kubebuilder:validation:Required
	// URL of the Git repository
	URL string `json:"url"`

	// +kubebuilder:validation:Required
	// Branch of the Git repository to monitor (TODO: rename to "Ref"; let user specify "refs/heads/...", "refs/tags/..." or SHA)
	Branch string `json:"branch"`

	// +kubebuilder:validation:MinLength=1
	// Polling interval for the Git repository
	PollingInterval string `json:"pollingInterval"`
}

// GitRepositoryStatus defines the observed state of GitRepository
type GitRepositoryStatus struct {
	// SHA of the last successfully applied commit
	LastPulledSHA string `json:"lastPulledSHA,omitempty"`

	// Directory where the Git repository is cloned
	WorkDirectory string `json:"workDirectory,omitempty"`

	// Conditions represent the latest available observations of the resource
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.url"
//+kubebuilder:printcolumn:name="Branch",type="string",JSONPath=".spec.branch"
//+kubebuilder:printcolumn:name="Interval",type="string",JSONPath=".spec.pollingInterval"
//+kubebuilder:printcolumn:name="SHA",type="string",JSONPath=".status.lastPulledSHA"

// GitRepository defines a single monitored Git repository
//go:generate go run ../../scripts/objecter/objecter.go -type=GitRepository
type GitRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitRepositorySpec   `json:"spec"`
	Status GitRepositoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GitRepositoryList contains a list of GitRepository
type GitRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitRepository{}, &GitRepositoryList{})
}
