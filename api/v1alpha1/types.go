package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SourceType selects where the desired manifests come from.
// +kubebuilder:validation:Enum=Git;ConfigMap;Helm;Kustomize
type SourceType string

const (
	// SourceTypeGit loads desired manifests from a Git repository.
	SourceTypeGit SourceType = "Git"
	// SourceTypeConfigMap loads desired manifests from a ConfigMap.
	SourceTypeConfigMap SourceType = "ConfigMap"
	// SourceTypeHelm renders a Helm chart (from a Git repository) in-process.
	SourceTypeHelm SourceType = "Helm"
	// SourceTypeKustomize builds a Kustomize overlay (from a Git repository)
	// in-process.
	SourceTypeKustomize SourceType = "Kustomize"
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

	// Auth references credentials for cloning a private repository. When unset,
	// the repository is cloned anonymously.
	Auth *GitAuth `json:"auth,omitempty"`
}

// GitAuthType selects the authentication scheme for cloning a private Git
// repository.
// +kubebuilder:validation:Enum=Basic;Bearer;SSH
type GitAuthType string

const (
	// GitAuthBasic authenticates over HTTPS with a username and password/token.
	GitAuthBasic GitAuthType = "Basic"
	// GitAuthBearer authenticates over HTTPS with a bearer token (e.g. a GitHub
	// App installation token or an OAuth token).
	GitAuthBearer GitAuthType = "Bearer"
	// GitAuthSSH authenticates over SSH with a private key.
	GitAuthSSH GitAuthType = "SSH"
)

// GitAuth configures authentication for cloning a private Git repository. The
// credentials are read from a Secret in the DriftCheck's namespace; the keys
// read depend on Type:
//   - Basic:  "username" and "password" (a personal access token goes in
//     "password").
//   - Bearer: "bearerToken".
//   - SSH:    "identity" (a PEM-encoded private key), "known_hosts" (required —
//     host-key verification is fail-closed), and optional "password" (the
//     private-key passphrase).
type GitAuth struct {
	// Type selects the authentication scheme.
	Type GitAuthType `json:"type"`

	// SecretRef names the Secret in the DriftCheck's namespace holding the
	// credentials.
	SecretRef LocalSecretRef `json:"secretRef"`
}

// LocalSecretRef names a Secret in the DriftCheck's namespace.
type LocalSecretRef struct {
	// Name of the Secret.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
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

// HelmSource renders a Helm chart, sourced from a Git repository, in-process
// (no `helm` binary or shell-out). The rendered manifests are compared against
// the live cluster.
type HelmSource struct {
	// Git locates the chart. Path points at the chart directory within the
	// repository (the directory containing Chart.yaml).
	Git GitSource `json:"git"`

	// ReleaseName is the release name used for rendering (`.Release.Name`).
	// Defaults to the DriftCheck's name.
	ReleaseName string `json:"releaseName,omitempty"`

	// Namespace is the release namespace used for rendering (`.Release.Namespace`).
	// Defaults to the DriftCheck's namespace.
	Namespace string `json:"namespace,omitempty"`

	// Values holds inline chart values that override the chart defaults and any
	// ValuesFiles.
	// +kubebuilder:pruning:PreserveUnknownFields
	Values *apiextensionsv1.JSON `json:"values,omitempty"`

	// ValuesFiles lists values files (relative to the chart directory) merged in
	// order before Values.
	ValuesFiles []string `json:"valuesFiles,omitempty"`
}

// KustomizeSource builds a Kustomize overlay, sourced from a Git repository,
// in-process (no `kustomize`/`kubectl` binary or shell-out).
type KustomizeSource struct {
	// Git locates the overlay. Path points at the directory holding the
	// kustomization.yaml within the repository.
	Git GitSource `json:"git"`
}

// Source describes the desired-state manifests to compare against the cluster.
// +kubebuilder:validation:XValidation:rule="self.type != 'ConfigMap' || has(self.configMap)",message="configMap is required when type is ConfigMap"
// +kubebuilder:validation:XValidation:rule="self.type != 'Git' || has(self.git)",message="git is required when type is Git"
// +kubebuilder:validation:XValidation:rule="self.type != 'Helm' || has(self.helm)",message="helm is required when type is Helm"
// +kubebuilder:validation:XValidation:rule="self.type != 'Kustomize' || has(self.kustomize)",message="kustomize is required when type is Kustomize"
type Source struct {
	// Type selects the source backend.
	Type SourceType `json:"type"`

	// Git configures a Git source. Required when Type is "Git".
	Git *GitSource `json:"git,omitempty"`

	// ConfigMap configures a ConfigMap source. Required when Type is "ConfigMap".
	ConfigMap *ConfigMapSource `json:"configMap,omitempty"`

	// Helm configures a Helm-chart source. Required when Type is "Helm".
	Helm *HelmSource `json:"helm,omitempty"`

	// Kustomize configures a Kustomize source. Required when Type is "Kustomize".
	Kustomize *KustomizeSource `json:"kustomize,omitempty"`
}

// WebhookType selects the payload format posted to a notification webhook.
// +kubebuilder:validation:Enum=Slack;Generic
type WebhookType string

const (
	// WebhookSlack posts a {"text": ...} message to a Slack incoming webhook.
	WebhookSlack WebhookType = "Slack"
	// WebhookGeneric posts a structured JSON body describing the drift.
	WebhookGeneric WebhookType = "Generic"
)

// SecretKeyRef selects a single key of a Secret in the DriftCheck's namespace.
type SecretKeyRef struct {
	// Name of the Secret.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key selects the entry within the Secret holding the value.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// Webhook is a single notification endpoint.
// +kubebuilder:validation:XValidation:rule="has(self.url) || has(self.urlSecretRef)",message="one of url or urlSecretRef is required"
type Webhook struct {
	// Type selects the payload format. Slack posts a {"text": ...} message;
	// Generic posts a structured JSON body.
	// +kubebuilder:default=Generic
	Type WebhookType `json:"type,omitempty"`

	// URL is the webhook endpoint. Prefer URLSecretRef for secret URLs such as
	// Slack incoming webhooks.
	URL string `json:"url,omitempty"`

	// URLSecretRef sources the webhook URL from a Secret in the DriftCheck's
	// namespace. It takes precedence over URL when both are set.
	URLSecretRef *SecretKeyRef `json:"urlSecretRef,omitempty"`
}

// NotifySpec configures drift notifications. A message is delivered to each
// webhook whenever the set of drifted resources changes (including when drift
// clears), not on every re-check.
type NotifySpec struct {
	// Webhooks receive a notification whenever the drift state changes.
	// +kubebuilder:validation:MinItems=1
	Webhooks []Webhook `json:"webhooks"`
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

	// Notify configures drift notifications. When unset, no webhooks are called.
	Notify *NotifySpec `json:"notify,omitempty"`
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

	// LastNotifiedHash fingerprints the drift set last delivered to the
	// configured webhooks. It prevents re-notifying on every re-check while the
	// drift state is unchanged; it is empty until the first notification.
	LastNotifiedHash string `json:"lastNotifiedHash,omitempty"`

	// Conditions represent the latest available observations.
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
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
