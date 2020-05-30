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

package helmrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"

	"helm.sh/helm/v3/pkg/chart/loader"
	storagev3 "helm.sh/helm/v3/pkg/storage"

	helmclient "github.com/operator-framework/operator-sdk/pkg/helm/client"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	driverv3 "helm.sh/helm/v3/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	"github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/utils"
)

//newHelmReleaseManagerFactory create a new manager returns a helmManagerFactory
func (r *ReconcileHelmRelease) newHelmReleaseManagerFactory(
	s *appv1.HelmRelease) (helmrelease.ManagerFactory, error) {

	chartDir, err := downloadChart(r.GetClient(), s)
	if err != nil {
		klog.Error(err, " - Failed to download the chart")
		return nil, err
	}

	klog.V(3).Info("ChartDir: ", chartDir)

	f := helmrelease.NewManagerFactory(r.Manager, chartDir)

	return f, nil
}

//newHelmReleaseManager create a new manager returns a helmManager
func (r *ReconcileHelmRelease) newHelmReleaseManager(
	s *appv1.HelmRelease, request reconcile.Request, factory helmrelease.ManagerFactory) (helmrelease.Manager, error) {
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(s.GroupVersionKind())
	o.SetNamespace(request.Namespace)
	o.SetName(request.Name)

	err := r.GetClient().Get(context.TODO(), request.NamespacedName, o)
	if err != nil {
		klog.Error(err, " - Failed to lookup resource")
		return nil, err
	}

	manager, err := factory.NewManager(o, nil)
	if err != nil {
		klog.Error(err, " - Failed to get release manager")
		return nil, err
	}

	return manager, nil
}

//downloadChart downloads the chart
func downloadChart(client client.Client, s *appv1.HelmRelease) (string, error) {
	configMap, err := utils.GetConfigMap(client, s.Namespace, s.Repo.ConfigMapRef)
	if err != nil {
		klog.Error(err)
		return "", err
	}

	secret, err := utils.GetSecret(client, s.Namespace, s.Repo.SecretRef)
	if err != nil {
		klog.Error(err, " - Failed to retrieve secret ", s.Repo.SecretRef.Name)
		return "", err
	}

	chartsDir := os.Getenv(appv1.ChartsDir)
	if chartsDir == "" {
		chartsDir, err = ioutil.TempDir("/tmp", "charts")
		if err != nil {
			klog.Error(err, " - Can not create tempdir")
			return "", err
		}
	}

	chartDir, err := utils.DownloadChart(configMap, secret, chartsDir, s)
	klog.V(3).Info("ChartDir: ", chartDir)

	if err != nil {
		klog.Error(err, " - Failed to download the chart")
		return "", err
	}

	return chartDir, nil
}

//GenerateManfiest generates the manifest for given HelmRelease
func GenerateManfiest(client client.Client, mgr crmanager.Manager, s *appv1.HelmRelease) (string, error) {
	chartDir, err := downloadChart(client, s)
	if err != nil {
		klog.Error(err, " - Failed to download the chart")
		return "", err
	}

	var values map[string]interface{}

	reqBodyBytes := new(bytes.Buffer)
	err = json.NewEncoder(reqBodyBytes).Encode(s.Spec)
	if err != nil {
		return "", err
	}

	err = yaml.Unmarshal([]byte(reqBodyBytes.Bytes()), &values)
	if err != nil {
		klog.Error(err, " - Failed to Unmarshal the spec ", s.Spec)
		return "", err
	}

	klog.V(3).Info("ChartDir: ", chartDir)

	chart, err := loader.LoadDir(chartDir)
	if err != nil {
		return "", fmt.Errorf("failed to load chart dir: %w", err)
	}

	clientv1, err := v1.NewForConfig(mgr.GetConfig())
	if err != nil {
		return "", fmt.Errorf("failed to get core/v1 client: %w", err)
	}

	storageBackendV3 := storagev3.Init(driverv3.NewSecrets(clientv1.Secrets(s.Namespace)))

	rcg, err := helmclient.NewRESTClientGetter(mgr, s.Namespace)
	if err != nil {
		return "", fmt.Errorf("failed to get REST client getter from manager: %w", err)
	}

	kubeClient := kube.New(rcg)
	ownerRef := metav1.NewControllerRef(s, s.GroupVersionKind())
	ownerRefClient := helmclient.NewOwnerRefInjectingClient(*kubeClient, *ownerRef)

	actionConfig := &action.Configuration{
		RESTClientGetter: rcg,
		Releases:         storageBackendV3,
		KubeClient:       ownerRefClient,
		Log:              func(_ string, _ ...interface{}) {},
	}

	install := action.NewInstall(actionConfig)
	install.ReleaseName = s.Name
	install.Namespace = s.Namespace
	install.DryRun = true

	release, err := install.Run(chart, values)
	if err != nil {
		return "", err
	}

	return release.Manifest, nil
}
