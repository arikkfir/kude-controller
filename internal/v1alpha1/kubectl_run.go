package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KubectlRunSpec defines the specification of the run
type KubectlRunSpec struct {
	Directory string   `json:"directory,omitempty"` // Local directory in the kude-controller pod where the command is executed
	Command   string   `json:"command,omitempty"`   // Executable (will be "kubectl")
	Args      []string `json:"args,omitempty"`      // Arguments passed to the command
}

// KubectlRunStatus defines the observed state of a KubectlRun.
type KubectlRunStatus struct {
	ExitCode int    `json:"exitCode,omitempty"` // Exit code of the "kubectl" command
	Output   string `json:"output,omitempty"`   // Combined output of stdout and stderr of the "kubectl" command
	Error    string `json:"error,omitempty"`    // Optional additional error message
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KubectlRun defines the complete definition of a kubectl run.
type KubectlRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubectlRunSpec   `json:"spec,omitempty"`
	Status KubectlRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubectlRunList contains a list of KubectlRun
type KubectlRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubectlRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubectlRun{}, &KubectlRunList{})
}
