/*
Copyright 2026 Ansible.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultVersion is the APME image tag shipped with this operator.
	DefaultVersion = "2026.8.6"
	// DefaultRegistry is the default pull registry for APME images.
	DefaultRegistry = "quay.io/ansible"
	// DefaultAbbenayImage is the Abbenay sidecar image when AI is enabled.
	DefaultAbbenayImage = "ghcr.io/redhat-developer/abbenay:v2026.8.7"
	// DefaultPostgresImage is an OpenShift-friendly arbitrary-UID Postgres.
	DefaultPostgresImage = "quay.io/sclorg/postgresql-16-c9s:latest"
	// FieldManager is the SSA field manager for owned objects.
	FieldManager = "apme-operator"
)

// DatabaseMode is the persistence mode the operator chose.
type DatabaseMode string

const (
	DatabaseManaged  DatabaseMode = "Managed"
	DatabaseExternal DatabaseMode = "External"
)

// Topology is the deployment shape. v1 always writes Simple.
type Topology string

const (
	TopologySimple Topology = "Simple"
)

// ApmeSpec defines the desired state of Apme.
//
// +kubebuilder:validation:XValidation:rule="!(has(self.components) && has(self.components.ui) && self.components.ui && has(self.exposure) && has(self.exposure.route) && self.exposure.route.enabled && (!has(self.exposure.route.host) || size(self.exposure.route.host) == 0))",message="exposure.route.host is required when UI and Route are enabled"
type ApmeSpec struct {
	// Version is the APME image tag (without a leading v). Defaults to 2026.8.6.
	// +optional
	Version string `json:"version,omitempty"`

	// Image configures registry, pull policy, and pull secrets.
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// Replicas of the Simple APME Deployment. v1 requires 1.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Components toggles optional sidecars/validators. Omitted fields default true.
	// +optional
	Components ComponentsSpec `json:"components,omitempty"`

	// Database selects managed Postgres (default) or an external Secret.
	// +optional
	Database DatabaseSpec `json:"database,omitempty"`

	// Storage sizes for sessions and Galaxy proxy cache PVCs.
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`

	// Exposure configures OpenShift Routes and optional Ingress.
	// +optional
	Exposure ExposureSpec `json:"exposure,omitempty"`

	// Abbenay is off by default (Helm parity).
	// +optional
	Abbenay AbbenaySpec `json:"abbenay,omitempty"`

	// Resources, when set, are applied to every APME container as a floor override.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// NetworkPolicy defaults to enabled.
	// +optional
	NetworkPolicy NetworkPolicySpec `json:"networkPolicy,omitempty"`
}

// ImageSpec is the shared APME image pull configuration.
type ImageSpec struct {
	// Registry defaults to quay.io/ansible. Images are {registry}/apme-{name}:{version}.
	// +optional
	Registry string `json:"registry,omitempty"`
	// PullPolicy defaults to IfNotPresent.
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
	// PullSecrets are imagePullSecrets on the APME and Postgres pods.
	// +optional
	PullSecrets []corev1.LocalObjectReference `json:"pullSecrets,omitempty"`
}

// ComponentsSpec toggles optional containers. Nil/omitted booleans default true.
type ComponentsSpec struct {
	// +optional
	Gitleaks *bool `json:"gitleaks,omitempty"`
	// +optional
	CollectionHealth *bool `json:"collectionHealth,omitempty"`
	// +optional
	DepAudit *bool `json:"depAudit,omitempty"`
	// +optional
	UI *bool `json:"ui,omitempty"`
}

// DatabaseSpec selects Managed vs External Postgres (#543 contract).
type DatabaseSpec struct {
	// ConnectionSecretRef, when name is set, is External mode. Key defaults to database-url.
	// +optional
	ConnectionSecretRef SecretKeyRef `json:"connectionSecretRef,omitempty"`
	// Postgres settings apply only in Managed mode.
	// +optional
	Postgres PostgresSpec `json:"postgres,omitempty"`
}

// SecretKeyRef names a Secret key. Empty Name means unset.
type SecretKeyRef struct {
	Name string `json:"name,omitempty"`
	Key  string `json:"key,omitempty"`
}

// PostgresSpec configures the operator-owned Postgres StatefulSet.
type PostgresSpec struct {
	// Image overrides the default sclorg Postgres image.
	// +optional
	Image string `json:"image,omitempty"`
	// Storage for the Postgres PVC.
	// +optional
	Storage PVCSpec `json:"storage,omitempty"`
	// Resources for the Postgres container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// StorageSpec is APME pod PVC sizes (no gateway-data / SQLite).
type StorageSpec struct {
	// +optional
	Sessions PVCSpec `json:"sessions,omitempty"`
	// +optional
	ProxyCache PVCSpec `json:"proxyCache,omitempty"`
}

// PVCSpec is size + optional storage class.
type PVCSpec struct {
	// +optional
	Size resource.Quantity `json:"size,omitempty"`
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// ExposureSpec is Route (OCP) and Ingress (vanilla k8s).
type ExposureSpec struct {
	// +optional
	Route RouteSpec `json:"route,omitempty"`
	// +optional
	Ingress IngressSpec `json:"ingress,omitempty"`
}

// RouteSpec is the OpenShift Route. Enabled defaults true.
type RouteSpec struct {
	// Enabled defaults true. Use a pointer so false is distinguishable.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Host is required when UI and Route are enabled.
	// +optional
	Host string `json:"host,omitempty"`
	// +optional
	TLS RouteTLSSpec `json:"tls,omitempty"`
}

// RouteTLSSpec is edge TLS on the Route.
type RouteTLSSpec struct {
	// +kubebuilder:default=edge
	// +optional
	Termination string `json:"termination,omitempty"`
	// +kubebuilder:default=Redirect
	// +optional
	InsecureEdgeTerminationPolicy string `json:"insecureEdgeTerminationPolicy,omitempty"`
}

// IngressSpec is optional vanilla Kubernetes Ingress.
type IngressSpec struct {
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// +optional
	ClassName string `json:"className,omitempty"`
	// +optional
	Host string `json:"host,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// +optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`
}

// AbbenaySpec is the optional AI sidecar.
type AbbenaySpec struct {
	// Enabled defaults false.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// +optional
	Image string `json:"image,omitempty"`
	// TokenSecretRef is optional; empty name means the operator generates a Secret once.
	// +optional
	TokenSecretRef SecretKeyRef `json:"tokenSecretRef,omitempty"`
	// ConfigMapRef optionally seeds Abbenay config.yaml.
	// +optional
	ConfigMapRef LocalObjectRef `json:"configMapRef,omitempty"`
	// Persistence defaults on (small PVC) when Abbenay is enabled.
	// +optional
	Persistence AbbenayPersistenceSpec `json:"persistence,omitempty"`
}

// AbbenayPersistenceSpec controls the Abbenay config PVC.
type AbbenayPersistenceSpec struct {
	// Enabled defaults true when Abbenay is on. Set false for emptyDir.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// +optional
	Size resource.Quantity `json:"size,omitempty"`
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// LocalObjectRef is a same-namespace object name.
type LocalObjectRef struct {
	Name string `json:"name,omitempty"`
}

// NetworkPolicySpec toggles operand NetworkPolicies. Enabled defaults true.
type NetworkPolicySpec struct {
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// ApmeStatus is the observed state of Apme.
type ApmeStatus struct {
	// Conditions: Ready, Progressing, Degraded, DatabaseReady.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// URL is the public UI (or API) URL.
	// +optional
	URL string `json:"url,omitempty"`
	// ObservedGeneration is the last spec generation reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Topology is always Simple in v1.
	// +optional
	Topology Topology `json:"topology,omitempty"`
	// Database is Managed or External.
	// +optional
	Database DatabaseMode `json:"database,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:path=apmes,scope=Namespaced,shortName=apme

// Apme is the Schema for the apmes API.
type Apme struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApmeSpec   `json:"spec,omitempty"`
	Status ApmeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApmeList contains a list of Apme.
type ApmeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Apme `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Apme{}, &ApmeList{})
}
