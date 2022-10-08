package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CommandRunSpec defines the specification of the run
type CommandRunSpec struct {
	CommitSHA string   `json:"commitSHA"`           // The commit SHA this command runs for
	Directory string   `json:"directory,omitempty"` // Local directory in the kude-controller pod where the command is executed
	Command   string   `json:"command,omitempty"`   // Executable (e.g. "kubectl")
	Args      []string `json:"args,omitempty"`      // Arguments passed to the command
}

// CommandRunStatus defines the observed state of a CommandRun.
type CommandRunStatus struct {
	ExitCode int    `json:"exitCode,omitempty"` // Exit code of the command
	Output   string `json:"output,omitempty"`   // Combined output of stdout and stderr of the command
	Error    string `json:"error,omitempty"`    // Optional additional error message
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CommandRun defines the complete definition of a command run.
type CommandRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CommandRunSpec   `json:"spec,omitempty"`
	Status CommandRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CommandRunList contains a list of CommandRun
type CommandRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CommandRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CommandRun{}, &CommandRunList{})
}
