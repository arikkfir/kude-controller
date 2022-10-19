package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HelmBundleSpec describes the desired state of a HelmBundle in the cluster. It provides the necessary information on
// the Helm chart to be installed in the cluster.
type HelmBundleSpec struct {
	Chart      string `json:"chart,omitempty"`      // Name of the Helm chart to be installed (without the "MyRepo/" prefix)
	Repository string `json:"repository,omitempty"` // Source repository of the Helm chart
	Release    string `json:"release,omitempty"`    // Name of the Helm release - use this to differentiate between multiple installations of the same chart in the cluster (e.g. "db1", "db2")
	Version    string `json:"version,omitempty"`    // Version of the Helm chart to use
	Values     string `json:"values,omitempty"`     // Custom values to provide as parameters to the Helm chart
}

// HelmBundleStatus defines the observed state of a HelmBundle.
type HelmBundleStatus struct {
	ChartStatus string `json:"chartStatus,omitempty"` // Status of the chart in the cluster

	// Conditions represent the latest available observations of the resource
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HelmBundle describes a bundle that installs a Helm chart into the cluster.
//go:generate go run ../../scripts/objecter/objecter.go -type=HelmBundle
type HelmBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmBundleSpec   `json:"spec,omitempty"`
	Status HelmBundleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmBundleList contains a list of helm bundles.
type HelmBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmBundle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmBundle{}, &HelmBundleList{})
}
