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
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-subscription-release/pkg/utils"
)

//newHelmReleaseManager create a new manager returns a helmManager
func (r *ReconcileHelmRelease) newHelmReleaseManager(
	s *appv1alpha1.HelmRelease) (helmrelease.Manager, error) {
	configMap, err := utils.GetConfigMap(r.GetClient(), s.Namespace, s.Spec.ConfigMapRef)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	secret, err := utils.GetSecret(r.GetClient(), s.Namespace, s.Spec.SecretRef)
	if err != nil {
		klog.Error(err, " - Failed to retrieve secret ", s.Spec.SecretRef.Name)
		return nil, err
	}

	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(s.GroupVersionKind())
	o.SetNamespace(s.GetNamespace())
	o.SetName(s.GetName())
	klog.V(2).Info("Name: ", o.GetName())
	o.SetUID(s.GetUID())
	klog.V(5).Info("uuid: ", o.GetUID())

	chartsDir := os.Getenv(appv1alpha1.ChartsDir)
	if chartsDir == "" {
		chartsDir, err = ioutil.TempDir("/tmp", "charts")
		if err != nil {
			klog.Error(err, " - Can not create tempdir")
			return nil, err
		}
	}

	chartDir, err := utils.DownloadChart(configMap, secret, chartsDir, s)
	klog.V(3).Info("ChartDir: ", chartDir)

	if s.DeletionTimestamp == nil {
		if err != nil {
			klog.Error(err, " - Failed to download the chart")
			return nil, err
		}

		if s.Spec.Values != "" {
			var spec interface{}

			err = yaml.Unmarshal([]byte(s.Spec.Values), &spec)
			if err != nil {
				klog.Error(err, " - Failed to Unmarshal the values ", s.Spec.Values)
				return nil, err
			}

			o.Object["spec"] = spec
		}
	} else if err != nil {
		//If error when download for deletion then create a fake chart.yaml.
		//The helmrelease manager needs only the name
		klog.Info("Unable to download ChartDir: ", chartDir, " creating a fake chart.yaml")
		chartDir, err = utils.CreateFakeChart(chartsDir, s)
		if err != nil {
			klog.Error(err, " - Failed to create fake chart for uninstall")
			return nil, err
		}
	}

	f := helmrelease.NewManagerFactory(r.Manager, chartDir)

	helmManager, err := f.NewManager(o)

	return helmManager, err
}
