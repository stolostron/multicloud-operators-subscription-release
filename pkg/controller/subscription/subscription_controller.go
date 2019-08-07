package subscription

import (
	"context"
	"encoding/json"
	gerrors "errors"
	"strings"

	"github.com/blang/semver"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/helm/pkg/repo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_subscription")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Subscription Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSubscription{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("subscription-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Subscription
	err = c.Watch(&source.Kind{Type: &appv1alpha1.Subscription{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Subscription
	err = c.Watch(&source.Kind{Type: &appv1alpha1.SubscriptionRelease{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.Subscription{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSubscription implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSubscription{}

// ReconcileSubscription reconciles a Subscription object
type ReconcileSubscription struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Subscription object and makes changes based on the state read
// and what is in the Subscription.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSubscription) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Subscription")

	// Fetch the Subscription instance
	instance := &appv1alpha1.Subscription{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Error reading the object - requeue the request")
		return reconcile.Result{}, err
	}

	// Define a new Pod object
	err = r.processSubscription(instance)
	if err != nil {
		reqLogger.Error(err, "Error processing subscription - requeue the request")
		return reconcile.Result{}, err
	}

	// Set Subscription instance as the owner and controller
	return reconcile.Result{}, nil
}

// do a helm repo subscriber
func (r *ReconcileSubscription) processSubscription(s *appv1alpha1.Subscription) error {
	subLogger := log.WithValues("Subscription.Namespace", s.Namespace, "Subscrption.Name", s.Name)
	httpClient, err := utils.GetHelmRepoClient()
	if err != nil {
		subLogger.Error(err, "Unable to create client for helm repo", "s.Spec.CatalogSource", s.Spec.CatalogSource)
		return err
	}
	//Retrieve the helm repo
	repoURL := s.Spec.CatalogSource
	log.Info("Source: " + repoURL)
	log.Info("name: " + s.GetName())

	indexFile, _, err := utils.GetHelmRepoIndex(s, httpClient, repoURL)
	if err != nil {
		subLogger.Error(err, "Unable to retrieve the helm repo index ", "s.Spec.CatalogSource", s.Spec.CatalogSource)
		return err
	}
	err = filterCharts(s, indexFile)
	if err != nil {
		subLogger.Error(err, "Unable to filter ", "s.Spec.CatalogSource", s.Spec.CatalogSource)
		return err
	}
	return r.manageSubscription(s, indexFile, repoURL)
}

//filterCharts filters the indexFile by name, tillerVersion, version, digest
func filterCharts(s *appv1alpha1.Subscription, indexFile *repo.IndexFile) (err error) {
	subLogger := log.WithValues("Subscription.Namespace", s.Namespace, "Subscrption.Name", s.Name)
	//Removes all entries from the indexFile with non matching name
	removeNoMatchingName(s, indexFile)
	//Removes non matching version, tillerVersion, digest
	filterOnVersion(s, indexFile)
	//Keep only the lastest version if multiple remains after filtering.
	err = takeLatestVersion(indexFile)
	if err != nil {
		subLogger.Error(err, "Failed to takeLatestVersion")
		return err
	}
	return nil
}

//removeNoMatchingName Deletes entries that the name doesn't match the name provided in the subscription
func removeNoMatchingName(s *appv1alpha1.Subscription, indexFile *repo.IndexFile) {
	if s != nil {
		if s.Spec.Package != "" {
			keys := make([]string, 0)
			for k := range indexFile.Entries {
				keys = append(keys, k)
			}
			for _, k := range keys {
				if k != s.Spec.Package {
					delete(indexFile.Entries, k)
				}
			}
		}
	}
}

//filterOnVersion filters the indexFile with the version, tillerVersion and Digest provided in the subscription
//The version provided in the subscription can be an expression like ">=1.2.3" (see https://github.com/blang/semver)
//The tillerVersion and the digest provided in the subscription must be literals.
func filterOnVersion(s *appv1alpha1.Subscription, indexFile *repo.IndexFile) {
	keys := make([]string, 0)
	for k := range indexFile.Entries {
		keys = append(keys, k)
	}
	for _, k := range keys {
		chartVersions := indexFile.Entries[k]
		newChartVersions := make([]*repo.ChartVersion, 0)
		for index, chartVersion := range chartVersions {
			if checkDigest(s, chartVersion) && checkTillerVersion(s, chartVersion) && checkVersion(s, chartVersion) {
				newChartVersions = append(newChartVersions, chartVersions[index])
			}
		}
		if len(newChartVersions) > 0 {
			indexFile.Entries[k] = newChartVersions
		} else {
			delete(indexFile.Entries, k)
		}
	}
}

//checkDigest Checks if the digest matches
func checkDigest(s *appv1alpha1.Subscription, chartVersion *repo.ChartVersion) bool {
	if s != nil {
		if s.Spec.PackageFilter != nil {
			if s.Spec.PackageFilter.Annotations != nil {
				if filterDigest, ok := s.Spec.PackageFilter.Annotations["digest"]; ok {
					return filterDigest == chartVersion.Digest
				}
			}
		}
	}
	return true

}

//checkTillerVersion Checks if the TillerVersion matches
func checkTillerVersion(s *appv1alpha1.Subscription, chartVersion *repo.ChartVersion) bool {
	subLogger := log.WithValues("Subscription.Namespace", s.Namespace, "Subscrption.Name", s.Name)
	if s != nil {
		if s.Spec.PackageFilter != nil {
			if s.Spec.PackageFilter.Annotations != nil {
				if filterTillerVersion, ok := s.Spec.PackageFilter.Annotations["tillerVersion"]; ok {
					tillerVersion := chartVersion.GetTillerVersion()
					if tillerVersion != "" {
						tillerVersionVersion, err := semver.ParseRange(tillerVersion)
						if err != nil {
							subLogger.Error(err, "Error while parsing", "tillerVersion: ", tillerVersion, " of ", chartVersion.GetName())
							return false
						}
						filterTillerVersion, err := semver.Parse(filterTillerVersion)
						if err != nil {
							subLogger.Error(err, "Failed to Parse ", filterTillerVersion)
							return false
						}
						return tillerVersionVersion(filterTillerVersion)
					}
				}
			}
		}
	}
	return true
}

//checkVersion checks if the version matches
func checkVersion(s *appv1alpha1.Subscription, chartVersion *repo.ChartVersion) bool {
	subLogger := log.WithValues("Subscription.Namespace", s.Namespace, "Subscrption.Name", s.Name)
	if s != nil {
		if s.Spec.PackageFilter != nil {
			if s.Spec.PackageFilter.Version != "" {
				version := chartVersion.GetVersion()
				versionVersion, err := semver.Parse(version)
				if err != nil {
					subLogger.Error(err, "Failed to parse ", version)
					return false
				}
				filterVersion, err := semver.ParseRange(s.Spec.PackageFilter.Version)
				if err != nil {
					subLogger.Error(err, "Failed to parse range ", "s.Spec.PackageFilter.Version", s.Spec.PackageFilter.Version)
					return false
				}
				return filterVersion(versionVersion)
			}
		}
	}
	return true
}

//takeLatestVersion if the indexFile contains multiple versions for a given chart, then
//only the latest is kept.
func takeLatestVersion(indexFile *repo.IndexFile) (err error) {
	indexFile.SortEntries()
	for k := range indexFile.Entries {
		//Get return the latest version when version is empty but
		//there is a bug in the masterminds semver used by helm
		// "*" constraint is not working properly
		// "*" is equivalent to ">=0.0.0"
		chartVersion, err := indexFile.Get(k, ">=0.0.0")
		if err != nil {
			log.Error(err, "Failed to get the latest version")
			return err
		}
		indexFile.Entries[k] = []*repo.ChartVersion{chartVersion}
	}
	return nil
}

func (r *ReconcileSubscription) manageSubscription(s *appv1alpha1.Subscription, indexFile *repo.IndexFile, repoURL string) error {
	subLogger := log.WithValues("Subscription.Namespace", s.Namespace, "Subscrption.Name", s.Name)
	//Loop on all packages selected by the subscription
	for _, chartVersions := range indexFile.Entries {
		if len(chartVersions) != 0 {
			sr, err := newSubscriptionReleaseForCR(s, chartVersions[0])
			if err != nil {
				return err
			}
			// Set SubscriptionRelease instance as the owner and controller
			if err := controllerutil.SetControllerReference(s, sr, r.scheme); err != nil {
				return err
			}
			// Check if this Pod already exists
			found := &appv1alpha1.SubscriptionRelease{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: sr.Name, Namespace: sr.Namespace}, found)
			if err != nil {
				if errors.IsNotFound(err) {
					subLogger.Info("Creating a new SubcriptionRelease", "SubcriptionRelease.Namespace", sr.Namespace, "SubcriptionRelease.Name", sr.Name)
					err = r.client.Create(context.TODO(), sr)
					if err != nil {
						return err
					}

				} else {
					return err
				}
			} else {
				subLogger.Info("Update a the SubcriptionRelease", "SubcriptionRelease.Namespace", sr.Namespace, "SubcriptionRelease.Name", sr.Name)
				sr.ObjectMeta = found.ObjectMeta
				err = r.client.Update(context.TODO(), sr)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newSubscriptionReleaseForCR(s *appv1alpha1.Subscription, chartVersion *repo.ChartVersion) (*appv1alpha1.SubscriptionRelease, error) {
	labels := map[string]string{
		"app":                   s.Name,
		"subscriptionName":      s.Name,
		"subscriptionNamespace": s.Namespace,
	}
	values, err := getValues(s, chartVersion)
	if err != nil {
		return nil, err
	}

	var channelName string
	if s.Spec.Channel != "" {
		strs := strings.Split(s.Spec.Channel, "/")
		if len(strs) != 2 {
			err = gerrors.New("Illegal channel settings, want namespace/name, but get " + s.Spec.Channel)
			return nil, err
		}
		channelName = strs[1]
	}

	releaseName := s.Name + "-" + chartVersion.Name
	if channelName != "" {
		releaseName = releaseName + "-" + channelName
	}
	//Compose release name
	sr := &appv1alpha1.SubscriptionRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseName,
			Namespace: s.Namespace,
			Labels:    labels,
		},
		Spec: appv1alpha1.SubscriptionReleaseSpec{
			URLs:        chartVersion.URLs,
			ChartName:   chartVersion.Name,
			ReleaseName: releaseName,
			Version:     chartVersion.GetVersion(),
			Values:      values,
		},
	}
	return sr, nil
}

func getValues(s *appv1alpha1.Subscription, chartVersion *repo.ChartVersion) (string, error) {
	for _, packageElem := range s.Spec.PackageOverrides {
		if packageElem.PackageName == chartVersion.Name {
			for _, pathElem := range packageElem.PackageOverrides {
				data, err := pathElem.MarshalJSON()
				if err != nil {
					return "", err
				}
				var m map[string]interface{}
				err = json.Unmarshal(data, &m)
				if err != nil {
					return "", err
				}
				if v, ok := m["path"]; ok && v == "spec.values" {
					return m["value"].(string), nil
				}
			}
		}
	}
	return "", nil
}
