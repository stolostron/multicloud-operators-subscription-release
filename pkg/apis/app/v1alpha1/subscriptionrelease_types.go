package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SubscriptionReleaseStatusEnum defines the status of a Subscription release
type SubscriptionReleaseStatusEnum string

const (
	// SubscriptionReleaseFailed means this subscription is the "parent" sitting in hub
	SubscriptionReleaseFailed  SubscriptionReleaseStatusEnum = "Failed"
	SubscriptionReleaseSuccess SubscriptionReleaseStatusEnum = "Success"
)

type SubscriptionReleaseStatus struct {
	Status         SubscriptionReleaseStatusEnum `json:"phase,omitempty"`
	Message        string                        `json:"message,omitempty"`
	Reason         string                        `json:"reason,omitempty"`
	LastUpdateTime metav1.Time                   `json:"lastUpdate"`
}

// SubscriptionReleaseSpec defines the desired state of SubscriptionRelease
// +k8s:openapi-gen=true
type SubscriptionReleaseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// RepoURL is the URL of the repository. Defaults to stable repo.
	URLs []string `json:"URLs,omitempty"`
	// ChartName is the name of the chart within the repo
	ChartName string `json:"chartName,omitempty"`
	// ReleaseName is the Name of the release given to Tiller. Defaults to namespace-name. Must not be changed after initial object creation.
	ReleaseName string `json:"releaseName,omitempty"`
	// Version is the chart version
	Version string `json:"version,omitempty"`
	// Values is a string containing (unparsed) YAML values
	Values string `json:"values,omitempty"`
	// Secret to use to access the helm-repo defined in the CatalogSource.
	SecretRef *corev1.ObjectReference `json:"secretRef,omitempty"`
	// Configuration parameters to access the helm-repo defined in the CatalogSource
	ConfigMapRef *corev1.ObjectReference `json:"configRef,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscriptionRelease is the Schema for the subscriptionreleases API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type SubscriptionRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubscriptionReleaseSpec   `json:"spec,omitempty"`
	Status SubscriptionReleaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscriptionReleaseList contains a list of SubscriptionRelease
type SubscriptionReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SubscriptionRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SubscriptionRelease{}, &SubscriptionReleaseList{})
}
