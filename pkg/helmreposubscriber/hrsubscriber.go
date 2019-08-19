// IBM Confidential
// OCO Source Materials
// 5737-E67
// (C) Copyright IBM Corporation 2019 All Rights Reserved
// The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.

package helmreposubscriber

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	gerrors "errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/ghodss/yaml"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/helm/pkg/repo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

//HelmRepoSubscriber the object thar represent a subscriber of a helmRepo
type HelmRepoSubscriber struct {
	Client       client.Client
	Scheme       *runtime.Scheme
	HelmRepoHash string
	Subscription *appv1alpha1.Subscription
	started      bool
	stopCh       chan struct{}
}

var log = logf.Log.WithName("helmreposubscriber")

var (
	subscriptionPeriod = 10 * time.Second
)

//DeploymentProcessHelmOperator value to use operator instead of bitnami as deployment tool
const DeploymentProcessHelmOperator = "helm-operator"

//DeploymentProcessBitnami value to use bitnami as deployment tool
const DeploymentProcessBitnami = "bitnami"

//DeploymentProcess
//var DeploymentProcess = DeploymentProcessHelmOperator

// Restart a helm repo subscriber
func (s *HelmRepoSubscriber) Restart() error {
	subLogger := log.WithValues("method", "Restart", "Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	subLogger.Info("begin")
	if s.started {
		s.Stop()
	}
	s.stopCh = make(chan struct{})

	s.HelmRepoHash = ""

	err := s.doSubscription()
	if err != nil {
		return err
	}

	subLogger.Info("Check start helm-repo monitoring", "s.Subscription.Spec.AutoUpgrade", s.Subscription.Spec.AutoUpgrade)
	if s.Subscription.Spec.AutoUpgrade {
		subLogger.Info("Start helm-repo monitoring")
		go wait.Until(func() {
			s.doSubscription()
		}, subscriptionPeriod, s.stopCh)
	}
	s.started = true

	return nil
}

// Stop a helm repo subscriber
func (s *HelmRepoSubscriber) Stop() error {
	subLogger := log.WithValues("method", "Stop", "Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	subLogger.Info("begin")
	close(s.stopCh)
	s.started = false
	return nil
}

// Update a namespace subscriber
func (s *HelmRepoSubscriber) Update(sub *appv1alpha1.Subscription) error {
	subLogger := log.WithValues("method", "Update", "Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	subLogger.Info("begin")
	s.Subscription = sub
	if !s.Subscription.Spec.AutoUpgrade {
		return s.Stop()
	}
	return s.Restart()
}

//IsStarted is true if subscriber started
func (s *HelmRepoSubscriber) IsStarted() bool {
	return s.started
}

//TODO
func (s *HelmRepoSubscriber) doSubscription() error {
	subLogger := log.WithValues("method", "doSubscription", "Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	subLogger.Info("start")
	//Retrieve the helm repo
	repoURL := s.Subscription.Spec.CatalogSource
	subLogger.Info("Source: " + repoURL)
	subLogger.Info("name: " + s.Subscription.GetName())

	indexFile, hash, err := s.GetHelmRepoIndex()
	if err != nil {
		subLogger.Error(err, "Unable to retrieve the helm repo index ", "s.Spec.CatalogSource", s.Subscription.Spec.CatalogSource)
		return err
	}
	if hash != s.HelmRepoHash {
		subLogger.Info("HelmRepo changed", "URL", repoURL)
		err = s.processSubscription(indexFile)
		if err != nil {
			subLogger.Error(err, "Error processing subscription")
			return err
		}
		s.HelmRepoHash = hash
	} else {
		subLogger.Info("HelmRepo didn't change", "URL", repoURL)
	}
	return nil
}

// do a helm repo subscriber
func (s *HelmRepoSubscriber) processSubscription(indexFile *repo.IndexFile) error {
	subLogger := log.WithValues("Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	//Retrieve the helm repo
	repoURL := s.Subscription.Spec.CatalogSource
	subLogger.Info("Source: " + repoURL)
	subLogger.Info("name: " + s.Subscription.GetName())

	err := s.filterCharts(indexFile)
	if err != nil {
		subLogger.Error(err, "Unable to filter ", "s.Spec.CatalogSource", s.Subscription.Spec.CatalogSource)
		return err
	}
	return s.manageSubscription(indexFile, repoURL)
}

//getHelmRepoIndex retreives the index.yaml, loads it into a repo.IndexFile and filters it
func (s *HelmRepoSubscriber) GetHelmRepoIndex() (indexFile *repo.IndexFile, hash string, err error) {
	subLogger := log.WithValues("Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	subLogger.Info("begin")
	configMap, err := utils.GetConfigMap(s.Client, s.Subscription.Namespace, s.Subscription.Spec.ConfigMapRef)
	if err != nil {
		subLogger.Error(err, "Failed to retrieve configMap ", "s.Spec.ConfigMapRef.Name", s.Subscription.Spec.ConfigMapRef.Name)
	}
	httpClient, err := utils.GetHelmRepoClient(s.Client, s.Subscription.Namespace, configMap)
	if err != nil {
		subLogger.Error(err, "Unable to create client for helm repo", "s.Spec.CatalogSource", s.Subscription.Spec.CatalogSource)
	}
	secret, err := utils.GetSecret(s.Client, s.Subscription.Namespace, s.Subscription.Spec.SecretRef)
	if err != nil {
		subLogger.Error(err, "Failed to retrieve secret ", "s.Spec.SecretRef.Name", s.Subscription.Spec.SecretRef.Name)
	}
	cleanRepoURL := strings.TrimSuffix(s.Subscription.Spec.CatalogSource, "/")
	req, err := http.NewRequest(http.MethodGet, cleanRepoURL+"/index.yaml", nil)
	if err != nil {
		subLogger.Error(err, "Can not build request: ", "cleanRepoURL", cleanRepoURL)
		return nil, "", err
	}
	if secret != nil && secret.Data != nil {
		if authHeader, ok := secret.Data["authHeader"]; ok {
			req.Header.Set("Authorization", string(authHeader))
		} else {
			if user, ok := secret.Data["user"]; ok {
				if password, ok := secret.Data["password"]; ok {
					req.SetBasicAuth(string(user), string(password))
				} else {
					return nil, "", fmt.Errorf("Password not found in secret for basic authentication")
				}
			}
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		subLogger.Error(err, "Http request failed: ", "cleanRepoURL", cleanRepoURL)
		return nil, "", err
	}
	subLogger.Info("Get suceeded", "cleanRepoURL", cleanRepoURL)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		subLogger.Error(err, "Unable to read body: ", "cleanRepoURL", cleanRepoURL)
		return nil, "", err
	}
	hash = hashKey(body)
	indexfile, err := LoadIndex(body)
	if err != nil {
		subLogger.Error(err, "Unable to parse the indexfile of ", "cleanRepoURL", cleanRepoURL)
		return nil, "", err
	}
	return indexfile, hash, err
}

//LoadIndex loads data into a repo.IndexFile
func LoadIndex(data []byte) (*repo.IndexFile, error) {
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
func (s *HelmRepoSubscriber) filterCharts(indexFile *repo.IndexFile) (err error) {
	subLogger := log.WithValues("Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	//Removes all entries from the indexFile with non matching name
	err = s.removeNoMatchingName(indexFile)
	if err != nil {
		subLogger.Error(err, "Failed to removeNoMatchingName")
		return err
	}
	//Removes non matching version, tillerVersion, digest
	s.filterIndexFile(indexFile)
	//Keep only the lastest version if multiple remains after filtering.
	err = s.takeLatestVersion(indexFile)
	if err != nil {
		subLogger.Error(err, "Failed to takeLatestVersion")
		return err
	}
	return nil
}

//removeNoMatchingName Deletes entries that the name doesn't match the name provided in the subscription
func (s *HelmRepoSubscriber) removeNoMatchingName(indexFile *repo.IndexFile) error {
	if s.Subscription != nil {
		if s.Subscription.Spec.Package != "" {
			r, err := regexp.Compile(s.Subscription.Spec.Package)
			if err != nil {
				return err
			}
			keys := make([]string, 0)
			for k := range indexFile.Entries {
				keys = append(keys, k)
			}
			for _, k := range keys {
				if !r.MatchString(k) {
					delete(indexFile.Entries, k)
				}
			}
		}
	}
	return nil
}

//filterIndexFile filters the indexFile with the version, tillerVersion and Digest provided in the subscription
//The version provided in the subscription can be an expression like ">=1.2.3" (see https://github.com/blang/semver)
//The tillerVersion and the digest provided in the subscription must be literals.
func (s *HelmRepoSubscriber) filterIndexFile(indexFile *repo.IndexFile) {
	keys := make([]string, 0)
	for k := range indexFile.Entries {
		keys = append(keys, k)
	}
	for _, k := range keys {
		chartVersions := indexFile.Entries[k]
		newChartVersions := make([]*repo.ChartVersion, 0)
		for index, chartVersion := range chartVersions {
			if s.checkDigest(chartVersion) && s.checkKeywords(chartVersion) && s.checkTillerVersion(chartVersion) && s.checkVersion(chartVersion) {
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

//checkKeywords Checks if the charts has at least 1 keyword from the packageFilter.Keywords array
func (s *HelmRepoSubscriber) checkKeywords(chartVersion *repo.ChartVersion) bool {
	if s.Subscription != nil {
		if s.Subscription.Spec.PackageFilter != nil {
			for _, filterKeyword := range s.Subscription.Spec.PackageFilter.Keywords {
				for _, chartKeyword := range chartVersion.Keywords {
					if filterKeyword == chartKeyword {
						return true
					}
				}
			}
			return false
		}
	}
	return true
}

//checkDigest Checks if the digest matches
func (s *HelmRepoSubscriber) checkDigest(chartVersion *repo.ChartVersion) bool {
	if s.Subscription != nil {
		if s.Subscription.Spec.PackageFilter != nil {
			if s.Subscription.Spec.PackageFilter.Annotations != nil {
				if filterDigest, ok := s.Subscription.Spec.PackageFilter.Annotations["digest"]; ok {
					return filterDigest == chartVersion.Digest
				}
			}
		}
	}
	return true

}

//checkTillerVersion Checks if the TillerVersion matches
func (s *HelmRepoSubscriber) checkTillerVersion(chartVersion *repo.ChartVersion) bool {
	subLogger := log.WithValues("Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	if s.Subscription != nil {
		if s.Subscription.Spec.PackageFilter != nil {
			if s.Subscription.Spec.PackageFilter.Annotations != nil {
				if filterTillerVersion, ok := s.Subscription.Spec.PackageFilter.Annotations["tillerVersion"]; ok {
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
func (s *HelmRepoSubscriber) checkVersion(chartVersion *repo.ChartVersion) bool {
	subLogger := log.WithValues("Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	if s.Subscription != nil {
		if s.Subscription.Spec.PackageFilter != nil {
			if s.Subscription.Spec.PackageFilter.Version != "" {
				version := chartVersion.GetVersion()
				versionVersion, err := semver.Parse(version)
				if err != nil {
					subLogger.Error(err, "Failed to parse ", version)
					return false
				}
				filterVersion, err := semver.ParseRange(s.Subscription.Spec.PackageFilter.Version)
				if err != nil {
					subLogger.Error(err, "Failed to parse range ", "s.Subscription.Spec.PackageFilter.Version", s.Subscription.Spec.PackageFilter.Version)
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
func (s *HelmRepoSubscriber) takeLatestVersion(indexFile *repo.IndexFile) (err error) {
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

func (s *HelmRepoSubscriber) manageSubscription(indexFile *repo.IndexFile, repoURL string) error {
	subLogger := log.WithValues("Subscription.Namespace", s.Subscription.Namespace, "Subscrption.Name", s.Subscription.Name)
	//Loop on all packages selected by the subscription
	for _, chartVersions := range indexFile.Entries {
		if len(chartVersions) != 0 {
			sr, err := s.newSubscriptionReleaseForCR(chartVersions[0])
			if err != nil {
				return err
			}
			// Set SubscriptionRelease instance as the owner and controller
			if err := controllerutil.SetControllerReference(s.Subscription, sr, s.Scheme); err != nil {
				return err
			}
			// Check if this Pod already exists
			found := &appv1alpha1.SubscriptionRelease{}
			err = s.Client.Get(context.TODO(), types.NamespacedName{Name: sr.Name, Namespace: sr.Namespace}, found)
			if err != nil {
				if errors.IsNotFound(err) {
					subLogger.Info("Creating a new SubcriptionRelease", "SubcriptionRelease.Namespace", sr.Namespace, "SubcriptionRelease.Name", sr.Name)
					err = s.Client.Create(context.TODO(), sr)
					if err != nil {
						return err
					}

				} else {
					return err
				}
			} else {
				subLogger.Info("Update a the SubcriptionRelease", "SubcriptionRelease.Namespace", sr.Namespace, "SubcriptionRelease.Name", sr.Name)
				sr.ObjectMeta = found.ObjectMeta
				err = s.Client.Update(context.TODO(), sr)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func (s *HelmRepoSubscriber) newSubscriptionReleaseForCR(chartVersion *repo.ChartVersion) (*appv1alpha1.SubscriptionRelease, error) {
	annotations := map[string]string{
		"app.ibm.com/hosting-deployable":   s.Subscription.Spec.Channel,
		"app.ibm.com/hosting-subscription": s.Subscription.Namespace + "/" + s.Subscription.Name,
	}
	values, err := s.getValues(chartVersion)
	if err != nil {
		return nil, err
	}

	var channelNamespace string
	var channelName string
	if s.Subscription.Spec.Channel != "" {
		strs := strings.Split(s.Subscription.Spec.Channel, "/")
		if len(strs) != 2 {
			err = gerrors.New("Illegal channel settings, want namespace/name, but get " + s.Subscription.Spec.Channel)
			return nil, err
		}
		channelNamespace = strs[0]
		channelName = strs[1]
	}

	releaseName := s.Subscription.Name + "-" + chartVersion.Name
	if channelName != "" {
		releaseName = releaseName + "-" + channelName
	}
	if channelNamespace != "" {
		releaseName = releaseName + "-" + channelNamespace
	}
	//Compose release name
	sr := &appv1alpha1.SubscriptionRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:        releaseName,
			Namespace:   s.Subscription.Namespace,
			Annotations: annotations,
		},
		Spec: appv1alpha1.SubscriptionReleaseSpec{
			URLs:         chartVersion.URLs,
			ConfigMapRef: s.Subscription.Spec.ConfigMapRef,
			SecretRef:    s.Subscription.Spec.SecretRef,
			ChartName:    chartVersion.Name,
			ReleaseName:  releaseName,
			Version:      chartVersion.GetVersion(),
			Values:       values,
		},
	}
	return sr, nil
}

func (s *HelmRepoSubscriber) getValues(chartVersion *repo.ChartVersion) (string, error) {
	for _, packageElem := range s.Subscription.Spec.PackageOverrides {
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
