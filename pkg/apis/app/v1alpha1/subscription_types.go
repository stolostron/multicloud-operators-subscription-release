package v1alpha1

import (
	operatorsv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	placementv1alpha1 "github.ibm.com/IBMMulticloudPlatform/placementrule/pkg/apis/app/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PackageFilter defines the reference to Channel
type PackageFilter struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	Keywords      []string              `json:"keywords,omitempty"`
	Annotations   map[string]string     `json:"annotations,omitempty"`
	// +kubebuilder:validation:Pattern=([0-9]+)((\.[0-9]+)(\.[0-9]+)|(\.[0-9]+)?(\.[xX]))$
	Version string `json:"version,omitempty"`
}

// PackageOverride describes rules for override
type PackageOverride struct {
	runtime.RawExtension `json:",inline"`
}

// Overrides field in deployable
type Overrides struct {
	PackageName string `json:"packageName"`
	// +kubebuilder:validation:MinItems=1
	PackageOverrides []PackageOverride `json:"packageOverrides"` // To be added
}

// SubscriptionSpec defines the desired state of Subscription
//// +k8s:openapi-gen=true
type SubscriptionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// RepoURL is the URL of the repository. Defaults to stable repo.

	// leverage and enhance subscription spec from operator lifecycle framework
	// mapping of the fields:
	// 	CatalogSourceNamespace		- For Namespace Channel only, namespace in hub, if specified ignore CatalogSource and Channel
	// 	CatalogSource				- For Helmrepo Channel only, url to helm repo, if specified, ignore Channel
	// 	Package						- Optional, to filter package by names
	// 	Channel						- Channel NamespacedName (in hub)
	// 	StartingCSV					- N/A
	// 	InstallPlanApproval			- N/A
	operatorsv1alpha1.SubscriptionSpec

	// To specify more than 1 package in channel
	PackageFilter *PackageFilter `json:"packageFilter,omitempty"`
	// To provide flexibility to override package in channel with local input
	PackageOverrides []*Overrides `json:"packageOverrides,omitempty"`
	// For hub use only, to specify which clusters to go to
	Placement *placementv1alpha1.Placement `json:"placement,omitempty"`
	// Secret to use to access the helm-repo defined in the CatalogSource.
	SecretRef *corev1.ObjectReference `json:"secretRef,omitempty"`
	// Configuration parameters to access the helm-repo defined in the CatalogSource
	ConfigMapRef *corev1.ObjectReference `json:"configRef,omitempty"`
	// AutoUpgrade if true the helm-repo will be monitor and subscription recalculated if changed
	AutoUpgrade bool `json:"autoUpgrade"`
}

// SubscriptionPhase defines the phasing of a Subscription
type SubscriptionPhase string

const (
	// SubscriptionPropagated means this subscription is the "parent" sitting in hub
	SubscriptionPropagated SubscriptionPhase = "Propagated"
	// SubscriptionSubscribed means this subscription is the "parent" sitting in hub
	SubscriptionSubscribed SubscriptionPhase = "Subscribed"
)

// SubscriptionStatus defines the observed state of Subscription
//// +k8s:openapi-gen=true
type SubscriptionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Phase are Propagated if it is in hub or Subscribed if it is in endpoint
	Phase   SubscriptionPhase `json:"phase,omitempty"`
	Message string            `json:"message,omitempty"`
	Reason  string            `json:"reason,omitempty"`

	// For endpoint, it is the status of subscription, For hub, it aggregates all status, key is cluster name
	Statuses map[string]operatorsv1alpha1.SubscriptionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Subscription is the Schema for the subscriptions API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Subscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubscriptionSpec   `json:"spec,omitempty"`
	Status SubscriptionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscriptionList contains a list of Subscription
type SubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subscription `json:"items"`
}

// Subscriber defines the interface for various channels
type Subscriber interface {
	Restart() error
	Stop() error
	Update(*Subscription) error
	IsStarted() bool
}

func init() {
	SchemeBuilder.Register(&Subscription{}, &SubscriptionList{})
}
