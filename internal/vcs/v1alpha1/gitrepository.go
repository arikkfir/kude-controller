package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitRepositorySpec defines the desired state of GitRepository
type GitRepositorySpec struct {
	URL string `json:"url,omitempty"` // URL of the Git repository
}

// GitRepositoryStatus defines the observed state of GitRepository
type GitRepositoryStatus struct {
	// TODO: Add observed state of the repository
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
