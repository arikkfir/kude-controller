package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitRepositorySpec is the desired state of a monitored Git repository.
type GitRepositorySpec struct {
	// +kubebuilder:validation:Required
	URL string `json:"url,omitempty"` // URL of the Git repository

	// +kubebuilder:validation:Required
	Branch string `json:"branch,omitempty"` // Branch of the Git repository to monitor

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PollingInterval string `json:"pollingInterval,omitempty"` // Polling interval for the Git repository
}

// GitRepositoryStatus defines the observed state of GitRepository
type GitRepositoryStatus struct {
	LastPulledSHA string             `json:"lastPulledSHA,omitempty"` // SHA of the last successfully applied commit
	WorkDirectory string             `json:"workDirectory,omitempty"` // Directory where the Git repository is cloned
	Conditions    []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GitRepository defines a single monitored Git repository
type GitRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitRepositorySpec   `json:"spec,omitempty"`
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
