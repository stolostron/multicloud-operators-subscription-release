package utils

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("utils")

//GetHelmRepoClient returns an *http.client to access the helm repo
func GetHelmRepoClient(client client.Client, parentNamespace string, configMap *corev1.ConfigMap) (*http.Client, error) {
	srLogger := log.WithValues("package", "utils", "method", "GetHelmRepoClient")

	httpClient := http.DefaultClient
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
		srLogger.Info("ConfigRef retrieved", "configMap.Data", configData)
		if configData["insecureSkipVerify"] != "" {
			b, err := strconv.ParseBool(configData["insecureSkipVerify"])
			if err != nil {
				if errors.IsNotFound(err) {
					return nil, nil
				}
				srLogger.Error(err, "Unable to parse", "insecureSkipVerify", configData["insecureSkipVerify"])
				return nil, err
			}
			srLogger.Info("Set InsecureSkipVerify", "insecureSkipVerify", b)
			transport.TLSClientConfig.InsecureSkipVerify = b
		} else {
			srLogger.Info("insecureSkipVerify is not specified")
		}
	} else {
		srLogger.Info("configMap is nil")
	}
	httpClient.Transport = transport
	srLogger.Info("InsecureSkipVerify equal", "InsecureSkipVerify", transport.TLSClientConfig.InsecureSkipVerify)
	return httpClient, nil
}

//GetConfigMap search the config map containing the helm repo client configuration.
func GetConfigMap(client client.Client, parentNamespace string, configMapRef *corev1.ObjectReference) (configMap *corev1.ConfigMap, err error) {
	srLogger := log.WithValues("package", "utils", "method", "getConfigMap")
	if configMapRef != nil {
		srLogger.Info("Retrieve configMap ", "parentNamespace", parentNamespace, "configMapRef.Name", configMapRef.Name)
		ns := configMapRef.Namespace
		if ns == "" {
			ns = parentNamespace
		}
		configMap = &corev1.ConfigMap{}
		err = client.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: configMapRef.Name}, configMap)
		if err != nil {
			if errors.IsNotFound(err) {
				srLogger.Error(err, "ConfigMap not found ", "Name:", configMapRef.Name, " on namespace: ", ns)
				return nil, nil
			}
			srLogger.Error(err, "Failed to get configMap ", "Name:", configMapRef.Name, " on namespace: ", ns)
			return nil, err
		}
		srLogger.Info("ConfigMap found ", "Name:", configMapRef.Name, " on namespace: ", ns)
	} else {
		srLogger.Info("no configMapRef defined ", "parentNamespace", parentNamespace)
	}
	return configMap, err
}

//GetSecret returns the secret to access the helm-repo
func GetSecret(client client.Client, parentNamespace string, secretRef *corev1.ObjectReference) (secret *corev1.Secret, err error) {
	srLogger := log.WithValues("package", "utils", "method", "getSecret")
	if secretRef != nil {
		srLogger.Info("Retreive secret", "parentNamespace", parentNamespace, "secretRef", secretRef)
		ns := secretRef.Namespace
		if ns == "" {
			ns = parentNamespace
		}
		secret = &corev1.Secret{}
		err = client.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: secretRef.Name}, secret)
		if err != nil {
			srLogger.Error(err, "Failed to get secret ", "Name:", secretRef.Name, " on namespace: ", secretRef.Namespace)
			return nil, err
		}
		srLogger.Info("Secret found ", "Name:", secretRef.Name, " on namespace: ", secretRef.Namespace)
	} else {
		srLogger.Info("No secret defined", "parentNamespace", parentNamespace)
	}
	return secret, err
}

//DownloadChart downloads a chart into the charsDir
func DownloadChart(httpClient *http.Client, secret *corev1.Secret, chartsDir string, s *appv1alpha1.SubscriptionRelease) (chartDir string, err error) {
	srLogger := log.WithValues("SubscriptionRelease.Namespace", s.Namespace, "SubscrptionRelease.Name", s.Name)
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
			if secret != nil && secret.Data != nil {
				req.SetBasicAuth(string(secret.Data["username"]), string(secret.Data["password"]))
			}
			resp, err := httpClient.Do(req)
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
	}
	return chartDir, err
}

//Untar untars the reader into the dst directory
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
