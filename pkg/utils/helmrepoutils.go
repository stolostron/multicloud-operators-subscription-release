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
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

//GetHelmRepoClient returns an *http.client to access the helm repo
func GetHelmRepoClient(parentNamespace string, configMap *corev1.ConfigMap) (*http.Client, error) {
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

//GetConfigMap search the config map containing the helm repo client configuration.
func GetConfigMap(client client.Client, parentNamespace string, configMapRef *corev1.ObjectReference) (configMap *corev1.ConfigMap, err error) {
	if configMapRef != nil {
		klog.V(5).Info("Retrieve configMap ", parentNamespace, "/", configMapRef.Name)
		ns := configMapRef.Namespace

		if ns == "" {
			ns = parentNamespace
		}

		configMap = &corev1.ConfigMap{}

		err = client.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: configMapRef.Name}, configMap)
		if err != nil {
			if errors.IsNotFound(err) {
				klog.Error(err, "ConfigMap not found ", "Name: ", configMapRef.Name, " on namespace: ", ns)
				return nil, nil
			}

			klog.Error(err, "Failed to get configMap ", "Name: ", configMapRef.Name, " on namespace: ", ns)

			return nil, err
		}

		klog.V(5).Info("ConfigMap found ", "Name:", configMapRef.Name, " on namespace: ", ns)
	} else {
		klog.V(5).Info("no configMapRef defined ", "parentNamespace", parentNamespace)
	}

	return configMap, err
}

//GetSecret returns the secret to access the helm-repo
func GetSecret(client client.Client, parentNamespace string, secretRef *corev1.ObjectReference) (secret *corev1.Secret, err error) {
	if secretRef != nil {
		klog.V(5).Info("retrieve secret :", parentNamespace, "/", secretRef)

		ns := secretRef.Namespace
		if ns == "" {
			ns = parentNamespace
		}

		secret = &corev1.Secret{}

		err = client.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: secretRef.Name}, secret)
		if err != nil {
			klog.Error(err, "Failed to get secret ", "Name: ", secretRef.Name, " on namespace: ", secretRef.Namespace)
			return nil, err
		}

		klog.V(5).Info("Secret found ", "Name: ", secretRef.Name, " on namespace: ", secretRef.Namespace)
	} else {
		klog.V(5).Info("No secret defined at ", "parentNamespace", parentNamespace)
	}

	return secret, err
}

func DownloadChart(configMap *corev1.ConfigMap, secret *corev1.Secret, chartsDir string, s *appv1alpha1.HelmRelease) (chartDir string, err error) {
	switch strings.ToLower(string(s.Spec.Source.SourceType)) {
	case string(appv1alpha1.HelmRepoSourceType):
		return DownloadChartFromHelmRepo(configMap, secret, chartsDir, s)
	case string(appv1alpha1.GitHubSourceType):
		return DownloadChartFromGitHub(configMap, secret, chartsDir, s)
	default:
		return "", fmt.Errorf("sourceType '%s' unsupported", s.Spec.Source.SourceType)
	}
}

//DownloadChartFromGitHub downloads a chart into the charsDir
func DownloadChartFromGitHub(configMap *corev1.ConfigMap, secret *corev1.Secret, chartsDir string, s *appv1alpha1.HelmRelease) (chartDir string, err error) {
	if s.Spec.Source.GitHub == nil {
		err := fmt.Errorf("github type but Spec.GitHub is not defined")
		return "", err
	}

	if _, err := os.Stat(chartsDir); os.IsNotExist(err) {
		err := os.MkdirAll(chartsDir, 0755)
		if err != nil {
			klog.Error(err, "Unable to create chartDir: ", chartsDir)
			return "", err
		}
	}

	destRepo := filepath.Join(chartsDir, s.Spec.ReleaseName, s.Namespace, s.Spec.ChartName)

	for _, url := range s.Spec.Source.GitHub.Urls {
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

		if s.Spec.Source.GitHub.Branch == "" {
			options.ReferenceName = plumbing.Master
		} else {
			options.ReferenceName = plumbing.ReferenceName(s.Spec.Source.GitHub.Branch)
		}

		os.RemoveAll(destRepo)

		_, err = git.PlainClone(destRepo, false, options)
		if err != nil {
			os.RemoveAll(destRepo)
			klog.Error(err, "Clone failed", "url", url)

			continue
		}
	}

	if err != nil {
		klog.Error(err, "All urls failed")
	}

	chartDir = filepath.Join(destRepo, s.Spec.Source.GitHub.ChartPath)

	return chartDir, err
}

//DownloadChartFromHelmRepo downloads a chart into the charsDir
func DownloadChartFromHelmRepo(configMap *corev1.ConfigMap,
	secret *corev1.Secret,
	chartsDir string,
	s *appv1alpha1.HelmRelease) (chartDir string, err error) {
	if s.Spec.Source.HelmRepo == nil {
		err := fmt.Errorf("helmrepo type but Spec.HelmRepo is not defined")
		return "", err
	}

	if _, err := os.Stat(chartsDir); os.IsNotExist(err) {
		err := os.MkdirAll(chartsDir, 0755)
		if err != nil {
			klog.Error(err, "Unable to create chartDir: ", "chartsDir", chartsDir)
			return "", err
		}
	}

	httpClient, err := GetHelmRepoClient(s.Namespace, configMap)
	if err != nil {
		klog.Error(err, "Failed to create httpClient sr.Spec.SecretRef.Name", s.Spec.SecretRef.Name)
		return "", err
	}

	var downloadErr error

	for _, urlelem := range s.Spec.Source.HelmRepo.Urls {
		var URLP *url.URL

		URLP, downloadErr = url.Parse(urlelem)
		if downloadErr != nil {
			klog.Error(downloadErr, "url", urlelem)
			continue
		}

		fileName := filepath.Base(URLP.Path)
		// Create the file
		chartZip := filepath.Join(chartsDir, fileName)
		if _, err := os.Stat(chartZip); os.IsNotExist(err) {
			var req *http.Request

			req, downloadErr = http.NewRequest(http.MethodGet, urlelem, nil)
			if downloadErr != nil {
				klog.Error(downloadErr, "Can not build request: ", "urlelem", urlelem)
				continue
			}

			if secret != nil && secret.Data != nil {
				req.SetBasicAuth(string(secret.Data["user"]), GetAccessToken(secret))
			}

			var resp *http.Response

			resp, downloadErr = httpClient.Do(req)
			if downloadErr != nil {
				klog.Error(downloadErr, "Http request failed: ", "urlelem", urlelem)
				continue
			}

			if resp.StatusCode != 200 {
				downloadErr = fmt.Errorf("return code: %d unable to retrieve chart", resp.StatusCode)
				klog.Error(downloadErr, "Unable to retrieve chart")

				continue
			}

			klog.V(5).Info("Download chart form helmrepo succeeded: ", urlelem)

			defer resp.Body.Close()

			var out *os.File

			out, downloadErr = os.Create(chartZip)
			if downloadErr != nil {
				klog.Error(downloadErr, "Failed to create: ", chartZip)
				continue
			}

			defer out.Close()

			// Write the body to file
			_, downloadErr = io.Copy(out, resp.Body)
			if downloadErr != nil {
				klog.Error(downloadErr, "Failed to copy body:", chartZip)
				continue
			}
		}

		var r *os.File

		r, downloadErr = os.Open(chartZip)
		if downloadErr != nil {
			klog.Error(downloadErr, "Failed to open: ", chartZip)
			continue
		}

		chartDirUnzip := filepath.Join(chartsDir, s.Spec.ReleaseName, s.Namespace)
		chartDir = filepath.Join(chartDirUnzip, s.Spec.ChartName)
		//Clean before untar
		os.RemoveAll(chartDirUnzip)

		downloadErr = Untar(chartDirUnzip, r)
		if downloadErr != nil {
			//Remove zip because failed to untar and so probably corrupted
			os.RemoveAll(chartZip)
			klog.Error(downloadErr, "Failed to unzip: ", chartZip)

			continue
		}
	}

	return chartDir, downloadErr
}

//DownloadGitHubRepo downloads a github repo into the charsDir
func DownloadGitHubRepo(configMap *corev1.ConfigMap,
	secret *corev1.Secret,
	chartsDir string,
	s *appv1alpha1.HelmChartSubscription) (destRepo string, commitID string, err error) {
	if s.Spec.Source.GitHub == nil {
		err := fmt.Errorf("github type but Spec.GitHub is not defined")
		return "", "", err
	}

	if _, err := os.Stat(chartsDir); os.IsNotExist(err) {
		err := os.MkdirAll(chartsDir, 0755)
		if err != nil {
			klog.Error(err, "Unable to create chartDir: ", chartsDir)
			return "", "", err
		}
	}

	destRepo = filepath.Join(chartsDir, s.Name, s.Namespace)

	for _, url := range s.Spec.Source.GitHub.Urls {
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

		if s.Spec.Source.GitHub.Branch == "" {
			options.ReferenceName = plumbing.Master
		} else {
			options.ReferenceName = plumbing.ReferenceName(s.Spec.Source.GitHub.Branch)
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

	return destRepo, commitID, err
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
