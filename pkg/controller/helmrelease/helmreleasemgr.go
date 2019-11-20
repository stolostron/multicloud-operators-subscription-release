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
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	helmrelease "github.com/operator-framework/operator-sdk/pkg/helm/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-subscription-release/pkg/utils"
)

//NewManager create a new manager
func newHelmReleaseManager(r *ReconcileHelmRelease,
	s *appv1alpha1.HelmRelease) (helmManager helmrelease.Manager, err error) {
	helmReleaseSecret, err := utils.GetSecret(r.client, s.Namespace, &corev1.ObjectReference{Name: s.Spec.ReleaseName})
	if err == nil {
		if !utils.IsOwned(s.ObjectMeta, helmReleaseSecret.ObjectMeta) {
			return nil,
				fmt.Errorf("duplicate release name: found existing release with name %q for another helmRelease %v",
					s.Spec.ReleaseName, helmReleaseSecret.GetOwnerReferences())
		}
	} else if errors.IsNotFound(err) {
		helmReleaseSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.Spec.ReleaseName,
				Namespace: s.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: appv1alpha1.SchemeGroupVersion.String(),
					Kind:       "HelmRelease",
					Name:       s.GetName(),
					UID:        s.GetUID(),
				}},
			},
			Type: corev1.SecretTypeOpaque,
		}
		err = r.client.Create(context.TODO(), helmReleaseSecret)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	configMap, err := utils.GetConfigMap(r.client, s.Namespace, s.Spec.ConfigMapRef)
	if err != nil {
		return nil, err
	}

	secret, err := utils.GetSecret(r.client, s.Namespace, s.Spec.SecretRef)
	if err != nil {
		klog.Error(err, "Failed to retrieve secret ", s.Spec.SecretRef.Name)
		return nil, err
	}

	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(helmReleaseSecret.GroupVersionKind())
	o.SetNamespace(helmReleaseSecret.GetNamespace())

	o.SetName(helmReleaseSecret.GetName())
	klog.V(2).Info("ReleaseName :", o.GetName())
	o.SetUID(helmReleaseSecret.GetUID())
	klog.V(5).Info("uuid:", o.GetUID())

	mgr, err := manager.New(r.config, manager.Options{
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
	klog.V(3).Info("ChartDir: ", chartDir)

	if s.DeletionTimestamp.IsZero() {
		if err != nil {
			klog.Error(err, "Failed to download the chart")
			return nil, err
		}

		if s.Spec.Values != "" {
			var spec interface{}

			err = yaml.Unmarshal([]byte(s.Spec.Values), &spec)
			if err != nil {
				klog.Error(err, "Failed to Unmarshal the values ", s.Spec.Values)
				return nil, err
			}

			o.Object["spec"] = spec
		}
	} else if err != nil {
		//If error when download for deletion then create a fake chart.yaml.
		//The helmrelease manager needs only the releaseName
		klog.Info("Unable to download ChartDir: ", chartDir, " creating a fake chart.yaml")
		chartDir, err = utils.CreateFakeChart(chartsDir, s)
		if err != nil {
			klog.Error(err, "Failed to create fake chart for uninstall")
			return nil, err
		}
	}

	f := helmrelease.NewManagerFactory(mgr, chartDir)

	helmManager, err = f.NewManager(o)

	return helmManager, err
}
