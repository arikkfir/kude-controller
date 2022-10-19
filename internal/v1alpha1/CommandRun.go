package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CommandRunSpec defines the specification of the run
type CommandRunSpec struct {
	// The commit SHA this command runs for
	CommitSHA string `json:"commitSHA"`

	// Local directory in the kude-controller pod where the command is executed
	Directory string `json:"directory"`

	// Executable (e.g. "kubectl")
	Command string `json:"command"`

	// Arguments passed to the command
	Args []string `json:"args"`
}

// CommandRunStatus defines the observed state of a CommandRun.
type CommandRunStatus struct {
	// Exit code of the command
	ExitCode int `json:"exitCode"`

	// Combined output of stdout and stderr of the command
	Output string `json:"output,omitempty"`

	// Optional additional error message
	Error string `json:"error,omitempty"`

	// Conditions of the command run
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Created",type="string",JSONPath=".metadata.creationTimestamp"
//+kubebuilder:printcolumn:name="Commit SHA",type="string",JSONPath=".spec.commitSHA"
//+kubebuilder:printcolumn:name="Command",type="string",JSONPath=".spec.args"
//+kubebuilder:printcolumn:name="Exit Code",type="string",JSONPath=".status.exitCode"
//+kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.error"

// CommandRun defines the complete definition of a command run.
//go:generate go run ../../scripts/objecter/objecter.go -type=CommandRun
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
