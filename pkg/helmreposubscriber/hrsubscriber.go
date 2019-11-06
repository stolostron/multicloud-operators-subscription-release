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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/repo"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-subscription-release/pkg/utils"
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

var (
	subscriptionPeriod = 10 * time.Second
)

//DeploymentProcessHelmOperator value to use operator instead of bitnami as deployment tool
const DeploymentProcessHelmOperator = "helm-operator"

//DeploymentProcessBitnami value to use bitnami as deployment tool
const DeploymentProcessBitnami = "bitnami"

// Restart a helm repo subscriber
func (s *HelmRepoSubscriber) Restart() error {
	klog.V(5).Info("Restart Subscriber")

	if s.started {
		err := s.Stop()
		if err != nil {
			return err
		}
	}

	s.HelmRepoHash = ""

	approval := strings.ToLower(string(s.HelmChartSubscription.Spec.InstallPlanApproval))
	klog.V(5).Info("Check start helm-repo monitoring",
		"s.HelmChartSubscription.Spec.InstallPlanApproval", s.HelmChartSubscription.Spec.InstallPlanApproval)

	if approval != "" && strings.EqualFold(approval, string(appv1alpha1.ApprovalAutomatic)) {
		klog.V(5).Info("Start helm-repo monitoring")

		s.stopCh = make(chan struct{})

		go wait.Until(func() {
			err := s.doHelmChartSubscription()
			if err != nil {
				klog.Error(err, "Error while managing the helmChartSubscription")
			}
		}, subscriptionPeriod, s.stopCh)

		s.started = true
	} else {
		s.started = false
		err := s.doHelmChartSubscription()
		if err != nil {
			return err
		}
	}

	return nil
}

// Stop a helm repo subscriber
func (s *HelmRepoSubscriber) Stop() error {
	if s.started {
		close(s.stopCh)
	}

	s.started = false

	return nil
}

// Update a namespace subscriber
func (s *HelmRepoSubscriber) Update(sub *appv1alpha1.HelmChartSubscription) error {
	s.HelmChartSubscription = sub
	approval := strings.ToLower(string(s.HelmChartSubscription.Spec.InstallPlanApproval))

	klog.V(5).Info("InstallPlanApproval: ", approval)
	klog.V(5).Info("ApprovalManual :", strings.ToLower(string(appv1alpha1.ApprovalManual)))

	if approval == "" || strings.EqualFold(string(s.HelmChartSubscription.Spec.InstallPlanApproval), string(appv1alpha1.ApprovalManual)) {
		return s.Stop()
	}

	return s.Restart()
}

//IsStarted is true if subscriber started
func (s *HelmRepoSubscriber) IsStarted() bool {
	return s.started
}

func (s *HelmRepoSubscriber) doHelmChartSubscription() error {
	//Retrieve the helm repo
	repoURL := s.HelmChartSubscription.Spec.Source.String()
	klog.V(5).Info("Source: ", repoURL)
	klog.Info("Subscription Name: ", s.HelmChartSubscription.GetName())

	var indexFile *repo.IndexFile

	var hash, url string

	var err error

	switch strings.ToLower(string(s.HelmChartSubscription.Spec.Source.SourceType)) {
	case string(appv1alpha1.HelmRepoSourceType):
		indexFile, hash, err = s.getHelmRepoIndex()
		url = fmt.Sprintf("%v", s.HelmChartSubscription.Spec.Source.HelmRepo.Urls)
	case string(appv1alpha1.GitHubSourceType):
		indexFile, hash, err = s.generateIndexYAML()
		url = fmt.Sprintf("%v", s.HelmChartSubscription.Spec.Source.GitHub.Urls)
	default:
		err = fmt.Errorf("sourceType '%s' unsupported", s.HelmChartSubscription.Spec.Source.SourceType)
	}

	if err != nil {
		klog.Error(err, "Unable to retrieve the helm repo index at ", url)
		return err
	}

	klog.V(5).Info(fmt.Sprintf("New hashes %s, old hash %s", hash, s.HelmRepoHash))

	if hash != s.HelmRepoHash {
		klog.Info("HelmRepo changed or subscription changed: ", repoURL)

		s.HelmRepoHash = hash

		err = s.processHelmChartSubscription(indexFile)
		if err != nil {
			klog.Error(err, "Error processing subscription")
			return err
		}
	} else {
		klog.Info("HelmRepo didn't change at ", repoURL)
	}

	return nil
}

// do a helm repo subscriber
func (s *HelmRepoSubscriber) processHelmChartSubscription(indexFile *repo.IndexFile) error {
	err := s.filterCharts(indexFile)
	if err != nil {
		klog.Error(err, "Unable to filter ")
		return err
	}

	return s.manageHelmChartSubscription(indexFile)
}

//getHelmRepoIndex retrieves the index.yaml, loads it into a repo.IndexFile and filters it
func (s *HelmRepoSubscriber) getHelmRepoIndex() (indexFile *repo.IndexFile, hash string, err error) {
	configMap, err := utils.GetConfigMap(s.Client, s.HelmChartSubscription.Namespace, s.HelmChartSubscription.Spec.ConfigMapRef)
	if err != nil {
		klog.Error(err, "Failed to retrieve configMap ", s.HelmChartSubscription.Spec.ConfigMapRef.Name)
	}

	httpClient, err := utils.GetHelmRepoClient(s.HelmChartSubscription.Namespace, configMap)
	if err != nil {
		klog.Error(err, "Unable to create client for helm repo",
			"s.HelmChartSubscription.Spec.Source.HelmRepo.Urls", s.HelmChartSubscription.Spec.Source.HelmRepo.Urls)
	}

	secret, err := utils.GetSecret(s.Client, s.HelmChartSubscription.Namespace, s.HelmChartSubscription.Spec.SecretRef)
	if err != nil {
		klog.Error(err, "Failed to retrieve secret ", s.HelmChartSubscription.Spec.SecretRef.Name)
	}

	for _, url := range s.HelmChartSubscription.Spec.Source.HelmRepo.Urls {
		cleanRepoURL := strings.TrimSuffix(url, "/")

		var req *http.Request

		req, err = http.NewRequest(http.MethodGet, cleanRepoURL+"/index.yaml", nil)
		if err != nil {
			klog.Error(err, "Can not build request: ", cleanRepoURL)
			continue
		}

		if secret != nil && secret.Data != nil {
			if authHeader, ok := secret.Data["authHeader"]; ok {
				req.Header.Set("Authorization", string(authHeader))
			} else if user, ok := secret.Data["user"]; ok {
				if token := utils.GetAccessToken(secret); token != "" {
					req.SetBasicAuth(string(user), token)
				} else {
					err = fmt.Errorf("no accessToken nor password found in secret for basic authentication")
					continue
				}
			}
		}

		var resp *http.Response

		resp, err = httpClient.Do(req)
		if err != nil {
			klog.Error(err, "Http request failed: ", "cleanRepoURL", cleanRepoURL)
			continue
		}

		if resp.StatusCode != 200 {
			err = fmt.Errorf("%s %s", resp.Status, cleanRepoURL+"/index.yaml")
			continue
		}

		klog.V(5).Info("Get index.yaml succeeded from ", cleanRepoURL)

		defer resp.Body.Close()

		var body []byte

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			klog.Error(err, "Unable to read body of ", cleanRepoURL)
			continue
		}

		hash, err = hashKey(body)
		if err != nil {
			klog.Error(err, "Unable to generate hashkey")
			continue
		}

		indexFile, err = LoadIndex(body)
		if err != nil {
			klog.Error(err, "Unable to parse the indexfile of ", cleanRepoURL)
			continue
		}
	}

	if err != nil {
		klog.Error(err, "All repo URL tested and all failed")
		return nil, "", err
	}

	return indexFile, hash, err
}

func (s *HelmRepoSubscriber) generateIndexYAML() (*repo.IndexFile, string, error) {
	configMap, err := utils.GetConfigMap(s.Client, s.HelmChartSubscription.Namespace, s.HelmChartSubscription.Spec.ConfigMapRef)
	if err != nil {
		return nil, "", err
	}

	secret, err := utils.GetSecret(s.Client, s.HelmChartSubscription.Namespace, s.HelmChartSubscription.Spec.SecretRef)
	if err != nil {
		klog.Error(err, "Failed to retrieve secret ", s.HelmChartSubscription.Spec.SecretRef.Name)
		return nil, "", err
	}

	chartsDir := os.Getenv(appv1alpha1.ChartsDir)
	if chartsDir == "" {
		chartsDir, err = ioutil.TempDir("/tmp", "charts")
		if err != nil {
			klog.Error(err, "Can not create tempdir")
			return nil, "", err
		}
	}

	repoRoot, hash, err := utils.DownloadGitHubRepo(configMap, secret, chartsDir, s.HelmChartSubscription)
	if err != nil {
		klog.Error(err, "Failed to download the repo")
		return nil, "", err
	}

	chartsPath := filepath.Join(repoRoot, s.HelmChartSubscription.Spec.Source.GitHub.ChartsPath)
	klog.V(3).Info("chartsPath: ", chartsPath)

	///////////////////////////////////////////////
	// Get chart directories first
	///////////////////////////////////////////////
	chartDirs := make(map[string]string)
	resourceDirs := make(map[string]string)

	currentChartDir := "NONE"

	err = filepath.Walk(chartsPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				klog.V(5).Info("Ignoring subfolders ", currentChartDir)
				if _, err := os.Stat(path + "/Chart.yaml"); err == nil {
					klog.Info("Found Chart.yaml in directory ", path)
					if !strings.HasPrefix(path, currentChartDir) {
						klog.V(5).Info("This is a helm chart folder.")
						chartDirs[path+"/"] = path + "/"
						currentChartDir = path + "/"
					}
				} else if !strings.HasPrefix(path, currentChartDir) && !strings.HasPrefix(path, repoRoot+"/.git") {
					klog.V(5).Info("This is not a helm chart directory ", path)
					resourceDirs[path+"/"] = path + "/"
				}
			}
			return nil
		})
	if err != nil {
		return nil, "", err
	}

	//////
	// Generate index.yaml
	/////

	keys := make([]string, 0)
	for k := range chartDirs {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	indexFile := repo.NewIndexFile()

	for _, k := range keys {
		chartDir := chartDirs[k]
		//	for chartDir := range chartDirs {
		chartFolderName := filepath.Base(chartDir)
		chartParentDir := strings.Split(chartDir, chartFolderName)[0]
		// Get the relative parent directory from the git repo root
		chartBaseDir := strings.SplitAfter(chartParentDir, chartsPath+"/")[1]

		chartMetadata, err := chartutil.LoadChartfile(chartDir + "Chart.yaml")
		if err != nil {
			klog.Error(err, "There was a problem in generating helm charts index file: ")
			return nil, "", err
		}

		if !indexFile.Has(chartMetadata.Name, chartMetadata.Version) {
			indexFile.Add(chartMetadata, chartFolderName, chartBaseDir, "generated-by-multicloud-operators-subscription")
		}
	}

	indexFile.SortEntries()

	b, _ := yaml.Marshal(indexFile)
	klog.V(5).Info("New index file content ", string(b), " with hash:", hash)

	return indexFile, hash, nil
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
func hashKey(b []byte) (string, error) {
	h := sha1.New()

	_, err := h.Write(b)
	if err != nil {
		return "", err
	}

	return string(h.Sum(nil)), nil
}

//filterCharts filters the indexFile by name, tillerVersion, version, digest
func (s *HelmRepoSubscriber) filterCharts(indexFile *repo.IndexFile) (err error) {
	//Removes all entries from the indexFile with non matching name
	err = s.removeNoMatchingName(indexFile)
	if err != nil {
		klog.Error(err, "Failed to removeNoMatchingName")
		return err
	}
	//Removes non matching version, tillerVersion, digest
	s.filterIndexFile(indexFile)
	//Keep only the lastest version if multiple remains after filtering.
	err = s.takeLatestVersion(indexFile)
	if err != nil {
		klog.Error(err, "Failed to takeLatestVersion")
		return err
	}

	return nil
}

//removeNoMatchingName Deletes entries that the name doesn't match the name provided in the subscription
func (s *HelmRepoSubscriber) removeNoMatchingName(indexFile *repo.IndexFile) error {
	if s.HelmChartSubscription != nil {
		if s.HelmChartSubscription.Spec.Package != "" {
			keys := make([]string, 0)
			for k := range indexFile.Entries {
				keys = append(keys, k)
			}

			for _, k := range keys {
				if k != s.HelmChartSubscription.Spec.Package {
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
	var labelSelector *metav1.LabelSelector
	if s.HelmChartSubscription.Spec.PackageFilter != nil {
		labelSelector = s.HelmChartSubscription.Spec.PackageFilter.LabelSelector
	}

	return utils.KeywordsChecker(labelSelector, chartVersion.Keywords)
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
	if s.HelmChartSubscription != nil {
		if s.HelmChartSubscription.Spec.PackageFilter != nil {
			if s.HelmChartSubscription.Spec.PackageFilter.Annotations != nil {
				if filterTillerVersion, ok := s.HelmChartSubscription.Spec.PackageFilter.Annotations["tillerVersion"]; ok {
					tillerVersion := chartVersion.GetTillerVersion()
					if tillerVersion != "" {
						tillerVersionVersion, err := semver.ParseRange(tillerVersion)
						if err != nil {
							klog.Error(err, "Error while parsing tillerVersion: ", tillerVersion, " of ", chartVersion.GetName())
							return false
						}

						filterTillerVersion, err := semver.Parse(filterTillerVersion)
						if err != nil {
							klog.Error(err, "Failed to Parse filterTillerVersion: ", filterTillerVersion)
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
	if s.HelmChartSubscription != nil {
		if s.HelmChartSubscription.Spec.PackageFilter != nil {
			if s.HelmChartSubscription.Spec.PackageFilter.Version != "" {
				version := chartVersion.GetVersion()

				versionVersion, err := semver.Parse(version)
				if err != nil {
					klog.Error(err, "Failed to parse version: ", version)
					return false
				}

				filterVersion, err := semver.ParseRange(s.HelmChartSubscription.Spec.PackageFilter.Version)
				if err != nil {
					klog.Error(err, "Failed to parse range ", s.HelmChartSubscription.Spec.PackageFilter.Version)
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
			klog.Error(err, "Failed to get the latest version")
			return err
		}

		indexFile.Entries[k] = []*repo.ChartVersion{chartVersion}
	}

	return nil
}

func (s *HelmRepoSubscriber) manageHelmChartSubscription(indexFile *repo.IndexFile) error {
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
					klog.Info("Creating a new HelmRelease: ", sr.Namespace, "/", sr.Name)

					err = s.Client.Create(context.TODO(), sr)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			} else {
				if !reflect.DeepEqual(found.Spec, sr.Spec) || found.Status.Status != appv1alpha1.HelmReleaseSuccess {
					klog.Info("Update a the HelmRelease: ", sr.Namespace, "/", sr.Name)
					klog.V(5).Info("found Spec: ", found.Spec)
					klog.V(5).Info("sr Spec", sr.Spec)
					found.Spec = sr.Spec

					err = s.Client.Update(context.TODO(), found)
					if err != nil {
						return err
					}
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
			Source:       &appv1alpha1.Source{},
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
		sr.Spec.Source.SourceType = appv1alpha1.HelmRepoSourceType
		sr.Spec.Source.HelmRepo = &appv1alpha1.HelmRepo{Urls: chartVersion.URLs}
	case string(appv1alpha1.GitHubSourceType):
		sr.Spec.Source.SourceType = appv1alpha1.GitHubSourceType
		sr.Spec.Source.GitHub = &appv1alpha1.GitHub{
			Urls:      s.HelmChartSubscription.Spec.Source.GitHub.Urls,
			Branch:    s.HelmChartSubscription.Spec.Source.GitHub.Branch,
			ChartPath: filepath.Join(s.HelmChartSubscription.Spec.Source.GitHub.ChartsPath, chartVersion.URLs[0]),
		}
	default:
		return nil, fmt.Errorf("sourceType '%s' unsupported", s.HelmChartSubscription.Spec.Source.SourceType)
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
