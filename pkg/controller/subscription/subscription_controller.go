package subscription

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/helmreleasemanager"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/helm/pkg/repo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
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
		return reconcile.Result{}, err
	}

	// Define a new Pod object
	r.processSubscription(instance)

	// Set Subscription instance as the owner and controller
	return reconcile.Result{}, nil
}

// do a helm repo subscriber
func (r *ReconcileSubscription) processSubscription(s *appv1alpha1.Subscription) {
	httpClient, err := getHelmRepoClient(s)
	if err != nil {
		glog.Error(err, "Unable to create client for helm repo", s.Spec.CatalogSource)
		return
	}
	//Retrieve the helm repo
	repoURL := s.Spec.CatalogSource
	log.Info("Source: " + repoURL)
	log.Info("name: " + s.GetName())

	indexFile, _, err := getHelmRepoIndex(s, httpClient, repoURL)
	if err != nil {
		glog.Error(err, "Unable to retrieve the helm repo index", s.Spec.CatalogSource)
		return
	}
	r.manageHelmCR(s, indexFile, repoURL)
}

func getHelmRepoClient(s *appv1alpha1.Subscription) (*http.Client, error) {
	client := http.DefaultClient
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client.Transport = transport
	return client, nil
}

//getHelmRepoIndex retreives the index.yaml, loads it into a repo.IndexFile and filters it
func getHelmRepoIndex(s *appv1alpha1.Subscription, client *http.Client, repoURL string) (indexFile *repo.IndexFile, hash string, err error) {
	cleanRepoURL := strings.TrimSuffix(repoURL, "/")
	req, err := http.NewRequest(http.MethodGet, cleanRepoURL+"/index.yaml", nil)
	if err != nil {
		glog.Error(err, "Can not build request: ", cleanRepoURL)
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		glog.Error(err, "Http request failed: ", cleanRepoURL)
		return nil, "", err
	}
	glog.Infof("Get %s suceeded: ", cleanRepoURL)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Error(err, "Unable to read body: ", cleanRepoURL)
		return nil, "", err
	}
	hash = hashKey(body)
	indexfile, err := loadIndex(body)
	if err != nil {
		glog.Error(err, "Unable to parse the indexfile: ", cleanRepoURL)
		return nil, "", err
	}
	err = filterCharts(s, indexfile)
	return indexfile, hash, err
}

//loadIndex loads data into a repo.IndexFile
func loadIndex(data []byte) (*repo.IndexFile, error) {
	i := &repo.IndexFile{}
	if err := yaml.Unmarshal(data, i); err != nil {
		return i, err
	}
	i.SortEntries()
	if i.APIVersion == "" {
		return i, repo.ErrNoAPIVersion
	}
	return i, nil
}

//hashKey Calculate a hash key
func hashKey(b []byte) string {
	h := sha1.New()
	h.Write(b)
	return string(h.Sum(nil))
}

//filterCharts filters the indexFile by name, tillerVersion, version, digest
func filterCharts(s *appv1alpha1.Subscription, indexFile *repo.IndexFile) (err error) {
	//Removes all entries from the indexFile with non matching name
	removeNoMatchingName(s, indexFile)
	//Removes non matching version, tillerVersion, digest
	filterOnVersion(s, indexFile)
	//Keep only the lastest version if multiple remains after filtering.
	err = takeLatestVersion(indexFile)
	if err != nil {
		glog.Error(err)
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
	if s != nil {
		if s.Spec.PackageFilter != nil {
			if s.Spec.PackageFilter.Annotations != nil {
				if filterTillerVersion, ok := s.Spec.PackageFilter.Annotations["tillerVersion"]; ok {
					tillerVersion := chartVersion.GetTillerVersion()
					if tillerVersion != "" {
						tillerVersionVersion, err := semver.ParseRange(tillerVersion)
						if err != nil {
							glog.Errorf("Error while parsing tillerVersion: %s of %s Error: %s", tillerVersion, chartVersion.GetName(), err.Error())
							return false
						}
						filterTillerVersion, err := semver.Parse(filterTillerVersion)
						if err != nil {
							glog.Error(err)
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
	if s != nil {
		if s.Spec.PackageFilter != nil {
			if s.Spec.PackageFilter.Version != "" {
				version := chartVersion.GetVersion()
				versionVersion, err := semver.Parse(version)
				if err != nil {
					glog.Error(err)
					return false
				}
				filterVersion, err := semver.ParseRange(s.Spec.PackageFilter.Version)
				if err != nil {
					glog.Error(err)
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
			glog.Error(err)
			return err
		}
		indexFile.Entries[k] = []*repo.ChartVersion{chartVersion}
	}
	return nil
}

func (r *ReconcileSubscription) manageHelmCR(s *appv1alpha1.Subscription, indexFile *repo.IndexFile, repoURL string) {
	// //Create the selector to find all HelmReleases generated by the subscription
	// labelSelector := fmt.Sprintf("subscriptionKind=%s, subscriptionName=%s", s.Kind, s.Name)
	// selector, err := labels.Parse(labelSelector)
	// if err != nil {
	// 	glog.Error("Unable to parse:", labelSelector, err)
	// 	return
	// }
	// //Retrieve the HelmRelease for the selector
	// helmReleases := &helmCrdV1.HelmReleaseList{}
	// errList := s.Client.List(context.TODO(),
	// 	&client.ListOptions{
	// 		Namespace:     s.Subscription.Namespace,
	// 		LabelSelector: selector,
	// 	},
	// 	helmReleases)
	// if errList != nil {
	// 	glog.Errorf("Unable to retreive resources described by %s, err: %s", labelSelector, errList.Error())
	// 	return
	// }
	// //Loop on the found helmReleases
	// for index, helmRelease := range helmReleases.Items {
	// 	//Check if the helmReleases is still selected by the subscription
	// 	if _, ok := indexFile.Entries[helmRelease.Spec.ChartName]; !ok {
	// 		//If not deleting it.
	// 		glog.Info("Deleting ", helmRelease)
	// 		err := s.Client.Delete(context.TODO(), &helmReleases.Items[index])
	// 		if err != nil {
	// 			glog.Error("Unable to delete ", helmRelease, err.Error())
	// 		}
	// 	}
	// }

	//Loop on all packages selected by the subscription
	for packageName, chartVersions := range indexFile.Entries {
		glog.Infof("chart: %s\n%v", packageName, chartVersions)
		mgr, err := helmreleasemanager.NewHelmManager(*s, chartVersions[0])
		if err != nil {
			glog.Error("Failed to create NewHelmManager ", packageName, " err:", err)
			continue
		}
		err = mgr.Sync(context.TODO())
		if err != nil {
			glog.Error("Failed to while sync ", packageName, " err:", err)
			continue
		}
		_, err = mgr.InstallRelease(context.TODO())
		if err != nil {
			glog.Error("Failed to while installing ", packageName, " err:", err)
			continue
		}
	}
}
