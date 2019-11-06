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

package helmreleasemgr

import (
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-subscription-release/pkg/utils"
)

//NewManager create a new manager
func NewHelmReleaseManager(cfg *rest.Config,
	configMap *corev1.ConfigMap,
	secret *corev1.Secret,
	s *appv1alpha1.HelmRelease) (helmManager helmrelease.Manager, err error) {
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(s.GroupVersionKind())
	o.SetNamespace(s.GetNamespace())

	releaseName := s.Spec.ReleaseName

	o.SetName(releaseName)
	klog.V(2).Info("ReleaseName :", o.GetName())
	o.SetUID(s.GetUID())
	klog.V(5).Info("uuid:", o.GetUID())

	mgr, err := manager.New(cfg, manager.Options{
		Namespace: s.GetNamespace(),
		//Disable MetricsListener
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Error(err, "Failed to create a new manager.")
		return nil, err
	}

	chartsDir := os.Getenv(appv1alpha1.ChartsDir)
	if chartsDir == "" {
		chartsDir, err = ioutil.TempDir("/tmp", "charts")
		if err != nil {
			klog.Error(err, "Can not create tempdir")
			return nil, err
		}
	}

	chartDir, err := utils.DownloadChart(configMap, secret, chartsDir, s)
	klog.Info("ChartDir: ", chartDir)

	if err != nil {
		klog.Error(err, "Failed to download the chart")
		return nil, err
	}

	f := helmrelease.NewManagerFactory(mgr, chartDir)

	if s.Spec.Values != "" {
		var spec interface{}

		err = yaml.Unmarshal([]byte(s.Spec.Values), &spec)
		if err != nil {
			klog.Error(err, "Failed to Unmarshal the values ", s.Spec.Values)
			return nil, err
		}

		o.Object["spec"] = spec
	}

	helmManager, err = f.NewManager(o)

	return helmManager, err
}
