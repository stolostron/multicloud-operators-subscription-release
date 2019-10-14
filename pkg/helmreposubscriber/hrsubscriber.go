/*
Copyright 2019 The Kubernetes Authors.

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

package helmreposubscriber

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	//	gerrors "errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	//	"regexp"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/ghodss/yaml"
	operatorsv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-subscription-release/pkg/utils"
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
	Client                client.Client
	Scheme                *runtime.Scheme
	HelmRepoHash          string
	HelmChartSubscription *appv1alpha1.HelmChartSubscription
	started               bool
	stopCh                chan struct{}
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
	subLogger := log.WithValues("method", "Restart", "HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
	subLogger.Info("begin")
	if s.started {
		s.Stop()
	}
	s.stopCh = make(chan struct{})

	s.HelmRepoHash = ""

	approval := strings.ToLower(string(s.HelmChartSubscription.Spec.InstallPlanApproval))
	subLogger.Info("Check start helm-repo monitoring", "s.HelmChartSubscription.Spec.InstallPlanApproval", s.HelmChartSubscription.Spec.InstallPlanApproval)
	if approval != "" && approval == strings.ToLower(string(operatorsv1alpha1.ApprovalAutomatic)) {
		subLogger.Info("Start helm-repo monitoring")
		go wait.Until(func() {
			s.doHelmChartSubscription()
		}, subscriptionPeriod, s.stopCh)
		s.started = true
	} else {
		err := s.doHelmChartSubscription()
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop a helm repo subscriber
func (s *HelmRepoSubscriber) Stop() error {
	subLogger := log.WithValues("method", "Stop", "HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
	subLogger.Info("begin")
	close(s.stopCh)
	s.started = false
	return nil
}

// Update a namespace subscriber
func (s *HelmRepoSubscriber) Update(sub *appv1alpha1.HelmChartSubscription) error {
	subLogger := log.WithValues("method", "Update", "HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
	subLogger.Info("begin")
	s.HelmChartSubscription = sub
	approval := strings.ToLower(string(s.HelmChartSubscription.Spec.InstallPlanApproval))
	subLogger.Info("InstallPlanApproval", "InstallPlanApproval", approval)
	subLogger.Info("ApprovalManual", "ApprovalManual", strings.ToLower(string(operatorsv1alpha1.ApprovalManual)))
	if approval == "" || strings.ToLower(string(s.HelmChartSubscription.Spec.InstallPlanApproval)) == strings.ToLower(string(operatorsv1alpha1.ApprovalManual)) {
		return s.Stop()
	}
	return s.Restart()
}

//IsStarted is true if subscriber started
func (s *HelmRepoSubscriber) IsStarted() bool {
	return s.started
}

//TODO
func (s *HelmRepoSubscriber) doHelmChartSubscription() error {
	subLogger := log.WithValues("method", "doHelmChartSubscription", "HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
	subLogger.Info("start")
	//Retrieve the helm repo
	if s.HelmChartSubscription.Spec.CatalogSource != "" {
		s.HelmChartSubscription.Spec.Source = &appv1alpha1.SourceSubscription{
			SourceType: appv1alpha1.HelmRepoSourceType,
			HelmRepo: &appv1alpha1.HelmRepoSubscription{
				Urls: []string{s.HelmChartSubscription.Spec.CatalogSource},
			},
		}
	}
	repoURL := s.HelmChartSubscription.Spec.Source.String()
	subLogger.Info("Source: " + repoURL)
	subLogger.Info("name: " + s.HelmChartSubscription.GetName())

	var indexFile *repo.IndexFile
	var hash, url string
	var err error

	switch strings.ToLower(string(s.HelmChartSubscription.Spec.Source.SourceType)) {
	case string(appv1alpha1.HelmRepoSourceType):
		indexFile, hash, err = s.GetHelmRepoIndex()
		url = fmt.Sprintf("%v", s.HelmChartSubscription.Spec.Source.HelmRepo.Urls)
	case string(appv1alpha1.GitHubSourceType):
		err = fmt.Errorf("Get IndexFile for sourceType '%s' not implemented", appv1alpha1.GitHubSourceType)
	default:
		err = fmt.Errorf("SourceType '%s' unsupported", s.HelmChartSubscription.Spec.Source.SourceType)
	}
	if err != nil {
		subLogger.Error(err, "Unable to retrieve the helm repo index ", "url", url)
		return err
	}
	subLogger.Info("Hashes", "hash", hash, "s.HelmRepoHash", s.HelmRepoHash)
	if hash != s.HelmRepoHash {
		subLogger.Info("HelmRepo changed or subscription changed", "URL", repoURL)
		err = s.processHelmChartSubscription(indexFile)
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
func (s *HelmRepoSubscriber) processHelmChartSubscription(indexFile *repo.IndexFile) error {
	subLogger := log.WithValues("HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)

	err := s.filterCharts(indexFile)
	if err != nil {
		subLogger.Error(err, "Unable to filter ")
		return err
	}
	return s.manageHelmChartSubscription(indexFile)
}

//GetHelmRepoIndex retreives the index.yaml, loads it into a repo.IndexFile and filters it
func (s *HelmRepoSubscriber) GetHelmRepoIndex() (indexFile *repo.IndexFile, hash string, err error) {
	subLogger := log.WithValues("HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
	subLogger.Info("begin")
	configMap, err := utils.GetConfigMap(s.Client, s.HelmChartSubscription.Namespace, s.HelmChartSubscription.Spec.ConfigMapRef)
	if err != nil {
		subLogger.Error(err, "Failed to retrieve configMap ", "s.Spec.ConfigMapRef.Name", s.HelmChartSubscription.Spec.ConfigMapRef.Name)
	}
	httpClient, err := utils.GetHelmRepoClient(s.HelmChartSubscription.Namespace, configMap)
	if err != nil {
		subLogger.Error(err, "Unable to create client for helm repo", "s.HelmChartSubscription.Spec.Source.HelmRepo.Urls", s.HelmChartSubscription.Spec.Source.HelmRepo.Urls)
	}
	secret, err := utils.GetSecret(s.Client, s.HelmChartSubscription.Namespace, s.HelmChartSubscription.Spec.SecretRef)
	if err != nil {
		subLogger.Error(err, "Failed to retrieve secret ", "s.Spec.SecretRef.Name", s.HelmChartSubscription.Spec.SecretRef.Name)
	}
	for _, url := range s.HelmChartSubscription.Spec.Source.HelmRepo.Urls {
		cleanRepoURL := strings.TrimSuffix(url, "/")
		var req *http.Request
		req, err = http.NewRequest(http.MethodGet, cleanRepoURL+"/index.yaml", nil)
		if err != nil {
			subLogger.Error(err, "Can not build request: ", "cleanRepoURL", cleanRepoURL)
			continue
		}
		if secret != nil && secret.Data != nil {
			if authHeader, ok := secret.Data["authHeader"]; ok {
				req.Header.Set("Authorization", string(authHeader))
			} else {
				if user, ok := secret.Data["user"]; ok {
					if password, ok := secret.Data["password"]; ok {
						req.SetBasicAuth(string(user), string(password))
					} else {
						err = fmt.Errorf("Password not found in secret for basic authentication")
						continue
					}
				}
			}
		}
		var resp *http.Response
		resp, err = httpClient.Do(req)
		if err != nil {
			subLogger.Error(err, "Http request failed: ", "cleanRepoURL", cleanRepoURL)
			continue
		}
		subLogger.Info("Get suceeded", "cleanRepoURL", cleanRepoURL)
		defer resp.Body.Close()
		var body []byte
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			subLogger.Error(err, "Unable to read body: ", "cleanRepoURL", cleanRepoURL)
			continue
		}
		hash = hashKey(body)
		indexFile, err = LoadIndex(body)
		if err != nil {
			subLogger.Error(err, "Unable to parse the indexfile of ", "cleanRepoURL", cleanRepoURL)
			continue
		}
	}
	if err != nil {
		subLogger.Error(err, "All repo URL tested and all failed")
		return nil, "", err
	}
	return indexFile, hash, err
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
	subLogger := log.WithValues("HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
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
	if s.HelmChartSubscription != nil {
		if s.HelmChartSubscription.Spec.Package != "" {
			// r, err := regexp.Compile(s.HelmChartSubscription.Spec.Package)
			// if err != nil {
			// 	return err
			// }
			keys := make([]string, 0)
			for k := range indexFile.Entries {
				keys = append(keys, k)
			}
			for _, k := range keys {
				if k != s.HelmChartSubscription.Spec.Package {
					// if !r.MatchString(k) {
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
	if s.HelmChartSubscription != nil {
		if s.HelmChartSubscription.Spec.PackageFilter != nil {
			if s.HelmChartSubscription.Spec.PackageFilter.Keywords == nil {
				return true
			}
			for _, filterKeyword := range s.HelmChartSubscription.Spec.PackageFilter.Keywords {
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
	if s.HelmChartSubscription != nil {
		if s.HelmChartSubscription.Spec.PackageFilter != nil {
			if s.HelmChartSubscription.Spec.PackageFilter.Annotations != nil {
				if filterDigest, ok := s.HelmChartSubscription.Spec.PackageFilter.Annotations["digest"]; ok {
					return filterDigest == chartVersion.Digest
				}
			}
		}
	}
	return true

}

//checkTillerVersion Checks if the TillerVersion matches
func (s *HelmRepoSubscriber) checkTillerVersion(chartVersion *repo.ChartVersion) bool {
	subLogger := log.WithValues("HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
	if s.HelmChartSubscription != nil {
		if s.HelmChartSubscription.Spec.PackageFilter != nil {
			if s.HelmChartSubscription.Spec.PackageFilter.Annotations != nil {
				if filterTillerVersion, ok := s.HelmChartSubscription.Spec.PackageFilter.Annotations["tillerVersion"]; ok {
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
	subLogger := log.WithValues("HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
	if s.HelmChartSubscription != nil {
		if s.HelmChartSubscription.Spec.PackageFilter != nil {
			if s.HelmChartSubscription.Spec.PackageFilter.Version != "" {
				version := chartVersion.GetVersion()
				versionVersion, err := semver.Parse(version)
				if err != nil {
					subLogger.Error(err, "Failed to parse ", version)
					return false
				}
				filterVersion, err := semver.ParseRange(s.HelmChartSubscription.Spec.PackageFilter.Version)
				if err != nil {
					subLogger.Error(err, "Failed to parse range ", "s.HelmChartSubscription.Spec.PackageFilter.Version", s.HelmChartSubscription.Spec.PackageFilter.Version)
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

func (s *HelmRepoSubscriber) manageHelmChartSubscription(indexFile *repo.IndexFile) error {
	subLogger := log.WithValues("HelmChartSubscription.Namespace", s.HelmChartSubscription.Namespace, "Subscrption.Name", s.HelmChartSubscription.Name)
	//Loop on all packages selected by the subscription
	for _, chartVersions := range indexFile.Entries {
		if len(chartVersions) != 0 {
			sr, err := s.newHelmChartHelmReleaseForCR(chartVersions[0])
			if err != nil {
				return err
			}
			// Set HelmChartHelmRelease instance as the owner and controller
			if err := controllerutil.SetControllerReference(s.HelmChartSubscription, sr, s.Scheme); err != nil {
				return err
			}
			// Check if this Pod already exists
			found := &appv1alpha1.HelmRelease{}
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

// newHelmChartHelmReleaseForCR
func (s *HelmRepoSubscriber) newHelmChartHelmReleaseForCR(chartVersion *repo.ChartVersion) (*appv1alpha1.HelmRelease, error) {
	annotations := map[string]string{
		"app.ibm.com/hosting-deployable":   s.HelmChartSubscription.Spec.Channel,
		"app.ibm.com/hosting-subscription": s.HelmChartSubscription.Namespace + "/" + s.HelmChartSubscription.Name,
	}
	values, err := s.getValues(chartVersion)
	if err != nil {
		return nil, err
	}

	releaseName := chartVersion.Name + "-" + s.HelmChartSubscription.Name + "-" + s.HelmChartSubscription.Namespace

	for i := range chartVersion.URLs {
		parsedURL, err := url.Parse(chartVersion.URLs[i])
		if err != nil {
			return nil, err
		}
		if parsedURL.Scheme == "local" {
			//make sure there is one and only one slash
			repoURL := strings.TrimSuffix(s.HelmChartSubscription.Spec.Source.HelmRepo.Urls[0], "/") + "/"
			chartVersion.URLs[i] = strings.Replace(chartVersion.URLs[i], "local://", repoURL, -1)
		}
	}
	//Compose release name
	sr := &appv1alpha1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:        releaseName,
			Namespace:   s.HelmChartSubscription.Namespace,
			Annotations: annotations,
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.HelmRepoSourceType,
			},
			ConfigMapRef: s.HelmChartSubscription.Spec.ConfigMapRef,
			SecretRef:    s.HelmChartSubscription.Spec.SecretRef,
			ChartName:    chartVersion.Name,
			ReleaseName:  releaseName,
			Version:      chartVersion.GetVersion(),
			Values:       values,
		},
	}
	switch strings.ToLower(string(s.HelmChartSubscription.Spec.Source.SourceType)) {
	case string(appv1alpha1.HelmRepoSourceType):
		sr.Spec.Source.HelmRepo = &appv1alpha1.HelmRepo{Urls: chartVersion.URLs}
	case string(appv1alpha1.GitHubSourceType):
		sr.Spec.Source.GitHub = &appv1alpha1.GitHub{
			Urls:      s.HelmChartSubscription.Spec.Source.GitHub.Urls,
			Branch:    s.HelmChartSubscription.Spec.Source.GitHub.Branch,
			ChartPath: chartVersion.URLs[0],
		}
	default:
		return nil, fmt.Errorf("SourceType '%s' unsupported", s.HelmChartSubscription.Spec.Source.SourceType)
	}
	return sr, nil
}

func (s *HelmRepoSubscriber) getValues(chartVersion *repo.ChartVersion) (string, error) {
	for _, packageElem := range s.HelmChartSubscription.Spec.PackageOverrides {
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
