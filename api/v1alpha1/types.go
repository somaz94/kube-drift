package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SourceType selects where the desired manifests come from.
// +kubebuilder:validation:Enum=Git;ConfigMap
type SourceType string

const (
	// SourceTypeGit loads desired manifests from a Git repository.
	SourceTypeGit SourceType = "Git"
	// SourceTypeConfigMap loads desired manifests from a ConfigMap.
	SourceTypeConfigMap SourceType = "ConfigMap"
)

// GitSource points at plain-YAML manifests in a Git repository.
type GitSource struct {
	// URL is the clone URL of the repository.
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// Ref is the branch, tag, or commit to check out. Defaults to the
	// repository's default branch when empty.
	Ref string `json:"ref,omitempty"`

	// Path is the directory within the repository holding the manifests.
	// Defaults to the repository root.
	Path string `json:"path,omitempty"`
}

// ConfigMapSource points at plain-YAML manifests stored in a ConfigMap.
type ConfigMapSource struct {
	// Name of the ConfigMap.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace of the ConfigMap. Defaults to the DriftCheck's namespace.
	Namespace string `json:"namespace,omitempty"`

	// Key selects a single ConfigMap data key. When empty, every key is
	// concatenated and parsed as a multi-document YAML stream.
	Key string `json:"key,omitempty"`
}

// Source describes the desired-state manifests to compare against the cluster.
type Source struct {
	// Type selects the source backend.
	Type SourceType `json:"type"`

	// Git configures a Git source. Required when Type is "Git".
	Git *GitSource `json:"git,omitempty"`

	// ConfigMap configures a ConfigMap source. Required when Type is "ConfigMap".
	ConfigMap *ConfigMapSource `json:"configMap,omitempty"`
}

// Target narrows which live resources the desired manifests are matched against.
// An empty Target matches by the identity (group/kind/namespace/name) carried in
// each desired manifest.
type Target struct {
	// Namespaces restricts comparison to these namespaces. Empty means the
	// namespace carried by each manifest is used as-is.
	Namespaces []string `json:"namespaces,omitempty"`

	// LabelSelector further restricts which desired manifests are compared.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// DriftCheckSpec defines the desired state of DriftCheck.
type DriftCheckSpec struct {
	// Source is where the desired-state manifests come from.
	Source Source `json:"source"`

	// Target narrows which resources are compared.
	Target Target `json:"target,omitempty"`

	// Interval is how often the drift check is re-evaluated.
	// +kubebuilder:default="5m"
	Interval metav1.Duration `json:"interval,omitempty"`
}

// DriftStatus mirrors the per-resource comparison outcome.
// +kubebuilder:validation:Enum=unchanged;changed;new;deleted
type DriftStatus string

const (
	DriftUnchanged DriftStatus = "unchanged"
	DriftChanged   DriftStatus = "changed"
	DriftNew       DriftStatus = "new"
	DriftDeleted   DriftStatus = "deleted"
)

// DriftedResource identifies a single resource that differs from its desired state.
type DriftedResource struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Name       string      `json:"name"`
	Namespace  string      `json:"namespace,omitempty"`
	Status     DriftStatus `json:"status"`
}

// DriftSummary tallies the comparison outcome across all compared resources.
type DriftSummary struct {
	Changed   int `json:"changed"`
	New       int `json:"new"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

// DriftCheckStatus defines the observed state of DriftCheck.
type DriftCheckStatus struct {
	// LastCheckedAt is when the drift check last completed.
	LastCheckedAt *metav1.Time `json:"lastCheckedAt,omitempty"`

	// DriftedResources lists resources whose live state differs from desired.
	// Unchanged resources are omitted; see Summary for totals.
	DriftedResources []DriftedResource `json:"driftedResources,omitempty"`

	// Summary tallies every compared resource by outcome.
	Summary DriftSummary `json:"summary,omitempty"`

	// ObservedGeneration is the .metadata.generation last reconciled.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Drifted",type=integer,JSONPath=`.status.summary.changed`
// +kubebuilder:printcolumn:name="New",type=integer,JSONPath=`.status.summary.new`
// +kubebuilder:printcolumn:name="Last Checked",type=date,JSONPath=`.status.lastCheckedAt`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DriftCheck is the Schema for the driftchecks API.
type DriftCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DriftCheckSpec   `json:"spec,omitempty"`
	Status DriftCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DriftCheckList contains a list of DriftCheck.
type DriftCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DriftCheck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DriftCheck{}, &DriftCheckList{})
}
