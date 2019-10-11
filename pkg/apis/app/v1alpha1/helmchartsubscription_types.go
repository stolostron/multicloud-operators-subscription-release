package v1alpha1

import (
	operatorsv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
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

// HelmChartSubscriptionSpec defines the desired state of HelmChartSubscription
//// +k8s:openapi-gen=true
type HelmChartSubscriptionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// RepoURL is the URL of the repository. Defaults to stable repo.
	// Source holds the url toward the helm-chart
	Source *Source `json:"chartsSource,omitempty"`

	// leverage and enhance subscription spec from operator lifecycle framework
	// mapping of the fields:
	// 	CatalogSourceNamespace		- N/A
	// 	CatalogSource				- if specified, ignore Source and will be a helm-repo
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
	//	Placement *placementv1alpha1.Placement `json:"placement,omitempty"`
	// Secret to use to access the helm-repo defined in the CatalogSource.
	SecretRef *corev1.ObjectReference `json:"secretRef,omitempty"`
	// Configuration parameters to access the helm-repo defined in the CatalogSource
	ConfigMapRef *corev1.ObjectReference `json:"configRef,omitempty"`

	Status HelmChartSubscriptionStatus `json:"status,omitempty"`
}

// HelmChartSubscriptionStatusEnum defines the status of a HelmChartSubscription
type HelmChartSubscriptionStatusEnum string

const (
	// HelmChartSubscriptionSuccess means this subscription Succeed
	HelmChartSubscriptionSuccess HelmChartSubscriptionStatusEnum = "Success"
	// HelmChartSubscriptionFailed means this subscription Failed
	HelmChartSubscriptionFailed HelmChartSubscriptionStatusEnum = "Failed"
)

// HelmChartSubscriptionUnitStatus defines status of a unit (subscription or package)
type HelmChartSubscriptionUnitStatus struct {
	// Phase are Propagated if it is in hub or Subscribed if it is in endpoint
	Status         HelmChartSubscriptionStatusEnum `json:"status,omitempty"`
	Message        string                          `json:"message,omitempty"`
	Reason         string                          `json:"reason,omitempty"`
	LastUpdateTime metav1.Time                     `json:"lastUpdateTime"`
}

// HelmChartSubscriptionStatus defines the observed state of HelmChartSubscription
//// +k8s:openapi-gen=true
type HelmChartSubscriptionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	HelmChartSubscriptionUnitStatus `json:",inline"`

	HelmChartSubscriptionPackageStatus map[string]HelmChartSubscriptionUnitStatus `json:"packages,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HelmChartSubscription is the Schema for the subscriptions API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type HelmChartSubscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmChartSubscriptionSpec   `json:"spec,omitempty"`
	Status HelmChartSubscriptionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HelmChartSubscriptionList contains a list of HelmChartSubscription
type HelmChartSubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmChartSubscription `json:"items"`
}

// Subscriber defines the interface for various channels
type Subscriber interface {
	Restart() error
	Stop() error
	Update(*HelmChartSubscription) error
	IsStarted() bool
}

func init() {
	SchemeBuilder.Register(&HelmChartSubscription{}, &HelmChartSubscriptionList{})
}
