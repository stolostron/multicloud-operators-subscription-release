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

package utils

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/repo"
	"k8s.io/klog"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

//GetHelmRepoClient returns an *http.client to access the helm repo
func GetHelmRepoClient(parentNamespace string, configMap *corev1.ConfigMap) (rest.HTTPClient, error) {
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
			InsecureSkipVerify: false,
		},
	}

	if configMap != nil {
		configData := configMap.Data
		klog.V(5).Info("ConfigRef retrieved :", configData)
		insecureSkipVerify := configData["insecureSkipVerify"]

		if insecureSkipVerify != "" {
			b, err := strconv.ParseBool(insecureSkipVerify)
			if err != nil {
				if errors.IsNotFound(err) {
					return nil, nil
				}

				klog.Error(err, "Unable to parse insecureSkipVerify", insecureSkipVerify)

				return nil, err
			}

			klog.V(5).Info("Set InsecureSkipVerify: ", b)
			transport.TLSClientConfig.InsecureSkipVerify = b
		} else {
			klog.V(5).Info("insecureSkipVerify is not specified")
		}
	} else {
		klog.V(5).Info("configMap is nil")
	}

	httpClient := http.DefaultClient
	httpClient.Transport = transport
	klog.V(5).Info("InsecureSkipVerify equal ", transport.TLSClientConfig.InsecureSkipVerify)

	return httpClient, nil
}

//DownloadChart downloads the charts
func DownloadChart(configMap *corev1.ConfigMap,
	secret *corev1.Secret,
	chartsDir string,
	s *appv1alpha1.HelmRelease) (chartDir string, err error) {
	destRepo := filepath.Join(chartsDir, s.Spec.ReleaseName, s.Namespace, s.Spec.ChartName)
	if _, err := os.Stat(destRepo); os.IsNotExist(err) {
		err := os.MkdirAll(destRepo, 0755)
		if err != nil {
			klog.Error(err, "Unable to create chartDir: ", destRepo)
			return "", err
		}
	}

	switch strings.ToLower(string(s.Spec.Source.SourceType)) {
	case string(appv1alpha1.HelmRepoSourceType):
		return DownloadChartFromHelmRepo(configMap, secret, destRepo, s)
	case string(appv1alpha1.GitHubSourceType):
		return DownloadChartFromGitHub(configMap, secret, destRepo, s)
	default:
		return "", fmt.Errorf("sourceType '%s' unsupported", s.Spec.Source.SourceType)
	}
}

//DownloadChartFromGitHub downloads a chart into the charsDir
func DownloadChartFromGitHub(configMap *corev1.ConfigMap, secret *corev1.Secret, destRepo string, s *appv1alpha1.HelmRelease) (chartDir string, err error) {
	if s.Spec.Source.GitHub == nil {
		err := fmt.Errorf("github type but Spec.GitHub is not defined")
		return "", err
	}

	_, err = DownloadGitHubRepo(configMap, secret, destRepo, s.Spec.Source.GitHub.Urls, s.Spec.Source.GitHub.Branch)

	if err != nil {
		return "", err
	}

	chartDir = filepath.Join(destRepo, s.Spec.Source.GitHub.ChartPath)

	return chartDir, err
}

//DownloadGitHubRepo downloads a github repo into the charsDir
func DownloadGitHubRepo(configMap *corev1.ConfigMap,
	secret *corev1.Secret,
	destRepo string,
	urls []string, branch string) (commitID string, err error) {
	for _, url := range urls {
		options := &git.CloneOptions{
			URL:               url,
			Depth:             1,
			SingleBranch:      true,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		}

		if secret != nil && secret.Data != nil {
			klog.V(5).Info("Add credentials")

			options.Auth = &githttp.BasicAuth{
				Username: string(secret.Data["user"]),
				Password: GetAccessToken(secret),
			}
		}

		if branch == "" {
			options.ReferenceName = plumbing.Master
		} else {
			options.ReferenceName = plumbing.ReferenceName("refs/heads/" + branch)
		}

		os.RemoveAll(destRepo)

		r, errClone := git.PlainClone(destRepo, false, options)

		if errClone != nil {
			os.RemoveAll(destRepo)
			klog.Error(errClone, "Clone failed: ", url)
			err = errClone

			continue
		}

		h, errHead := r.Head()

		if errHead != nil {
			os.RemoveAll(destRepo)
			klog.Error(errHead, "Get Head failed: ", url)
			err = errHead

			continue
		}

		commitID = h.Hash().String()
		klog.V(5).Info("commitID: ", commitID)
	}

	if err != nil {
		klog.Error(err, "All urls failed")
	}

	return commitID, err
}

//DownloadChartFromHelmRepo downloads a chart into the charsDir
func DownloadChartFromHelmRepo(configMap *corev1.ConfigMap,
	secret *corev1.Secret,
	destRepo string,
	s *appv1alpha1.HelmRelease) (chartDir string, err error) {
	if s.Spec.Source.HelmRepo == nil {
		err := fmt.Errorf("helmrepo type but Spec.HelmRepo is not defined")
		return "", err
	}

	var downloadErr error

	for _, urlelem := range s.Spec.Source.HelmRepo.Urls {
		chartZip, downloadErr := downloadFile(s.Namespace, configMap, urlelem, secret, destRepo)
		if downloadErr != nil {
			klog.Error(downloadErr, "url", urlelem)
			continue
		}

		var r *os.File

		r, downloadErr = os.Open(chartZip)
		if downloadErr != nil {
			klog.Error(downloadErr, "Failed to open: ", chartZip)
			continue
		}

		chartDir = filepath.Join(destRepo, s.Spec.ChartName)
		//Clean before untar
		os.RemoveAll(chartDir)

		downloadErr = Untar(destRepo, r)
		if downloadErr != nil {
			//Remove zip because failed to untar and so probably corrupted
			os.RemoveAll(chartZip)
			klog.Error(downloadErr, "Failed to unzip: ", chartZip)

			continue
		}
	}

	return chartDir, downloadErr
}

//downloadFile downloads a files and post it in the chartsDir.
func downloadFile(parentNamespace string, configMap *corev1.ConfigMap,
	fileURL string,
	secret *corev1.Secret,
	chartsDir string) (string, error) {
	klog.V(4).Info("fileURL: ", fileURL)

	URLP, downloadErr := url.Parse(fileURL)
	if downloadErr != nil {
		klog.Error(downloadErr, " url:", fileURL)
		return "", downloadErr
	}

	fileName := filepath.Base(URLP.RequestURI())
	klog.V(4).Info("fileName: ", fileName)
	// Create the file
	chartZip := filepath.Join(chartsDir, fileName)
	klog.V(4).Info("chartZip: ", chartZip)

	switch URLP.Scheme {
	case "file":
		downloadErr = downloadFileLocal(URLP, chartZip)
	case "http", "https":
		downloadErr = downloadFileHTTP(parentNamespace, configMap, fileURL, secret, chartZip)
	default:
		downloadErr = fmt.Errorf("unsupported scheme %s", URLP.Scheme)
	}

	return chartZip, downloadErr
}

func downloadFileLocal(urlP *url.URL,
	chartZip string) error {
	sourceFile, downloadErr := os.Open(urlP.RequestURI())
	if downloadErr != nil {
		klog.Error(downloadErr, " urlP.RequestURI:", urlP.RequestURI())
		return downloadErr
	}

	defer sourceFile.Close()

	// Create new file
	newFile, downloadErr := os.Create(chartZip)
	if downloadErr != nil {
		klog.Error(downloadErr, " chartZip:", chartZip)
		return downloadErr
	}

	defer newFile.Close()

	_, downloadErr = io.Copy(newFile, sourceFile)
	if downloadErr != nil {
		klog.Error(downloadErr)
		return downloadErr
	}

	return nil
}

func downloadFileHTTP(parentNamespace string, configMap *corev1.ConfigMap,
	fileURL string,
	secret *corev1.Secret,
	chartZip string) error {
	if _, err := os.Stat(chartZip); os.IsNotExist(err) {
		httpClient, downloadErr := GetHelmRepoClient(parentNamespace, configMap)
		if downloadErr != nil {
			klog.Error(downloadErr, "Failed to create httpClient")
			return downloadErr
		}

		var req *http.Request

		req, downloadErr = http.NewRequest(http.MethodGet, fileURL, nil)
		if downloadErr != nil {
			klog.Error(downloadErr, "Can not build request: ", "fileURL", fileURL)
			return downloadErr
		}

		if secret != nil && secret.Data != nil {
			req.SetBasicAuth(string(secret.Data["user"]), GetPassword(secret))
		}

		var resp *http.Response

		resp, downloadErr = httpClient.Do(req)
		if downloadErr != nil {
			klog.Error(downloadErr, "Http request failed: ", "fileURL", fileURL)
			return downloadErr
		}

		if resp.StatusCode != 200 {
			downloadErr = fmt.Errorf("return code: %d unable to retrieve chart", resp.StatusCode)
			klog.Error(downloadErr, "Unable to retrieve chart")

			return downloadErr
		}

		klog.V(5).Info("Download chart form helmrepo succeeded: ", fileURL)

		defer resp.Body.Close()

		var out *os.File

		out, downloadErr = os.Create(chartZip)
		if downloadErr != nil {
			klog.Error(downloadErr, "Failed to create: ", chartZip)
			return downloadErr
		}

		defer out.Close()

		// Write the body to file
		_, downloadErr = io.Copy(out, resp.Body)
		if downloadErr != nil {
			klog.Error(downloadErr, "Failed to copy body:", chartZip)
			return downloadErr
		}
	}

	return nil
}

//Untar untars the reader into the dst directory
func Untar(dst string, r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		klog.Error(err)
		return err
	}

	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF: // if no more files are found return
			return nil
		case err != nil: // return any other error
			klog.Error(err)
			return err
		case header == nil: // if the header is nil, just skip it (not sure how this happens)
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {
		case tar.TypeDir: // if its a dir and it doesn't exist create it
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					klog.Error(err)
					return err
				}
			}
		case tar.TypeReg: // if it's a file create it
			klog.V(3).Info("Untar to target :", target)

			dir := filepath.Dir(target)
			if _, err := os.Stat(dir); err != nil {
				if err := os.MkdirAll(dir, 0755); err != nil {
					klog.Error(err)
					return err
				}
			}

			if _, err := os.Stat(target); err == nil {
				klog.Info(fmt.Sprintf("A previous version exist of %s then delete", target))
				os.Remove(target)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				klog.Error(err)
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				klog.Error(err)
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

func KeywordsChecker(labelSelector *metav1.LabelSelector, ks []string) bool {
	ls := make(map[string]string)
	for _, k := range ks {
		ls[k] = "true"
	}

	return LabelsChecker(labelSelector, ls)
}

//UnmarshalIndex loads data into a repo.IndexFile
func UnmarshalIndex(data []byte) (*repo.IndexFile, error) {
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

func GenerateGitHubIndexFile(configMap *corev1.ConfigMap,
	secret *corev1.Secret,
	destDir string,
	urls []string,
	chartsPath string,
	branch string) (indexFile *repo.IndexFile, hash string, err error) {
	hash, err = DownloadGitHubRepo(configMap, secret, destDir, urls, branch)
	if err != nil {
		klog.Error(err, "Failed to download the repo")
		return nil, "", err
	}

	chartsPath = filepath.Join(destDir, chartsPath)
	klog.V(3).Info("chartsPath: ", chartsPath)

	indexFile, err = generateIndexFile(chartsPath)
	if err != nil {
		klog.Error(err, "Can not generate index file")
		return nil, "", err
	}

	b, _ := yaml.Marshal(indexFile)
	klog.V(5).Info("New index file content ", string(b), " with hash:", hash)

	return indexFile, hash, nil
}

func generateIndexFile(chartsPath string) (*repo.IndexFile, error) {
	///////////////////////////////////////////////
	// Get chart directories first
	///////////////////////////////////////////////
	chartDirs := make(map[string]string)

	currentChartDir := "NONE"

	err := filepath.Walk(chartsPath,
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
				}
			}
			return nil
		})
	if err != nil {
		return nil, err
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
			return nil, err
		}

		if !indexFile.Has(chartMetadata.Name, chartMetadata.Version) {
			indexFile.Add(chartMetadata, chartFolderName, chartBaseDir, "generated-by-multicloud-operators-subscription")
		}
	}

	indexFile.SortEntries()

	return indexFile, nil
}

//GetHelmRepoIndex retrieves the index.yaml, loads it into a repo.IndexFile and filters it
func GetHelmRepoIndex(configMap *corev1.ConfigMap,
	secret *corev1.Secret,
	parentNamespace string,
	urls []string) (indexFile *repo.IndexFile, hash string, err error) {
	httpClient, err := GetHelmRepoClient(parentNamespace, configMap)
	if err != nil {
		klog.Error(err, "Unable to create client for helm repo",
			"urls", urls)
	}

	for _, url := range urls {
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
				if password := GetPassword(secret); password != "" {
					req.SetBasicAuth(string(user), password)
				} else {
					err = fmt.Errorf("password found in secret for basic authentication")
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

		hash, err = HashKey(body)
		if err != nil {
			klog.Error(err, "Unable to generate hashkey")
			continue
		}

		indexFile, err = UnmarshalIndex(body)
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

//HashKey Calculate a hash key
func HashKey(b []byte) (string, error) {
	h := sha1.New()

	_, err := h.Write(b)
	if err != nil {
		return "", err
	}

	return string(h.Sum(nil)), nil
}

//CreateFakeChart Creates a fake Chart.yaml with the release name
func CreateFakeChart(chartsDir string, s *appv1alpha1.HelmRelease) (chartDir string, err error) {
	dirName := filepath.Join(chartsDir, s.Spec.ChartName)
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		err := os.MkdirAll(dirName, 0755)
		if err != nil {
			klog.Error(err, "Unable to create chartDir: ", dirName)
			return "", err
		}
	}

	fileName := filepath.Join(dirName, "Chart.yaml")
	chart := &chart.Metadata{
		Name: s.Spec.ReleaseName,
	}

	return dirName, chartutil.SaveChartfile(fileName, chart)
}
