package utils

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	"k8s.io/helm/pkg/repo"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("utils")

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

func GetHelmRepoClient() (*http.Client, error) {
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
func GetHelmRepoIndex(s *appv1alpha1.Subscription, client *http.Client, repoURL string) (indexFile *repo.IndexFile, hash string, err error) {
	subLogger := log.WithValues("Subscription.Namespace", s.Namespace, "Subscrption.Name", s.Name)
	cleanRepoURL := strings.TrimSuffix(repoURL, "/")
	req, err := http.NewRequest(http.MethodGet, cleanRepoURL+"/index.yaml", nil)
	if err != nil {
		subLogger.Error(err, "Can not build request: ", "cleanRepoURL", cleanRepoURL)
		return nil, "", err
	}
	resp, err := client.Do(req)
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

//hashKey Calculate a hash key
func hashKey(b []byte) string {
	h := sha1.New()
	h.Write(b)
	return string(h.Sum(nil))
}

func DownloadChart(chartsDir string, s appv1alpha1.SubscriptionRelease) (chartDir string, err error) {
	srLogger := log.WithValues("SubscriptionRelease.Namespace", s.Namespace, "SubscrptionRelease.Name", s.Name)
	client, err := GetHelmRepoClient()
	if err != nil {
		srLogger.Error(err, "Unable to create helm repo client: ")
		return "", err
	}
	if _, err := os.Stat(chartsDir); os.IsNotExist(err) {
		err := os.MkdirAll(chartsDir, 0755)
		if err != nil {
			srLogger.Error(err, "Unable to create chartDir: ", "chartsDir", chartsDir)
			return "", err
		}
	}
	for _, urlelem := range s.Spec.URLs {
		URLP, err := url.Parse(urlelem)
		if err != nil {
			return "", err
		}
		fileName := filepath.Base(URLP.Path)
		// Create the file
		chartZip := filepath.Join(chartsDir, fileName)
		if _, err := os.Stat(chartZip); os.IsNotExist(err) {
			req, err := http.NewRequest(http.MethodGet, urlelem, nil)
			if err != nil {
				srLogger.Error(err, "Can not build request: ", "urlelem", urlelem)
				return "", err
			}
			resp, err := client.Do(req)
			if err != nil {
				srLogger.Error(err, "Http request failed: ", "urlelem", urlelem)
				return "", err
			}
			srLogger.Info("Get suceeded: ", "urlelem", urlelem)
			defer resp.Body.Close()
			out, err := os.Create(chartZip)
			if err != nil {
				return "", err
			}
			defer out.Close()

			// Write the body to file
			_, err = io.Copy(out, resp.Body)
		}
		r, err := os.Open(chartZip)
		if err != nil {
			srLogger.Error(err, "Failed to open: ", "chartZip", chartZip)
			return "", err
		}
		chartDirUnzip := filepath.Join(chartsDir, s.Spec.ReleaseName, s.Namespace)
		chartDir = filepath.Join(chartDirUnzip, s.Spec.ChartName)
		os.RemoveAll(chartDirUnzip)
		err = Untar(chartDirUnzip, r)
		if err != nil {
			srLogger.Error(err, "Failed to unzip: ", "chartZip", chartZip)
			return "", err
		}
		//Add values.yaml file
		valuesFileName := filepath.Join(chartDir, "values.yaml")
		if s.Spec.Values != "" {
			data, err := ioutil.ReadFile(valuesFileName)
			if err != nil {
				srLogger.Error(err, "Failed to read values from  ", "valuesFileName", valuesFileName)
				return "", err
			}
			var valuesChart interface{}
			err = yaml.Unmarshal(data, &valuesChart)
			if err != nil {
				srLogger.Error(err, "Failed to Unmarshal values from  ", "valuesFileName", valuesFileName)
				return "", err
			}
			var valuesSub interface{}
			err = yaml.Unmarshal([]byte(s.Spec.Values), &valuesSub)
			if err != nil {
				srLogger.Error(err, "Failed to Unmarshal values from  subscriptionRelease")
				return "", err
			}
			values, err := merge(valuesSub, valuesChart)
			if err != nil {
				srLogger.Error(err, "Failed to merge values from  subscriptionRelease and from ", "valuesFileName", valuesFileName)
				return "", err
			}
			data, err = yaml.Marshal(values)
			if err != nil {
				srLogger.Error(err, "Failed to marshal merged values")
				return "", err
			}
			err = ioutil.WriteFile(valuesFileName, data, 0644)
			if err != nil {
				srLogger.Error(err, "Failed to write values to  ", "valuesFileName", valuesFileName)
				return "", err
			}
		}
	}
	return chartDir, err
}

func Untar(dst string, r io.Reader) error {

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			dir := filepath.Dir(target)
			if _, err := os.Stat(dir); err != nil {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

func merge(x1, x2 interface{}) (interface{}, error) {
	data1, err := json.Marshal(x1)
	if err != nil {
		return nil, err
	}
	data2, err := json.Marshal(x2)
	if err != nil {
		return nil, err
	}
	var j1 interface{}
	err = json.Unmarshal(data1, &j1)
	if err != nil {
		return nil, err
	}
	var j2 interface{}
	err = json.Unmarshal(data2, &j2)
	if err != nil {
		return nil, err
	}
	return merge1(j1, j2), nil
}

func merge1(x1, x2 interface{}) interface{} {
	switch x1 := x1.(type) {
	case map[string]interface{}:
		x2, ok := x2.(map[string]interface{})
		if !ok {
			return x1
		}
		for k, v2 := range x2 {
			if v1, ok := x1[k]; ok {
				x1[k] = merge1(v1, v2)
			} else {
				x1[k] = v2
			}
		}
	case []interface{}:
		x2, ok := x2.([]interface{})
		if !ok {
			return x1
		}
		for i := range x2 {
			x1 = append(x1, x2[i])
		}
		return x1
	case nil:
		// merge(nil, map[string]interface{...}) -> map[string]interface{...}
		x2, ok := x2.(map[string]interface{})
		if ok {
			return x2
		}
	}
	return x1
}
