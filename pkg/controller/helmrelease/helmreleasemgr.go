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
	"strings"

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

//newHelmReleaseManager create a new manager returns a helmManager and the new created secret
func newHelmReleaseManager(
	r *ReconcileHelmRelease,
	s *appv1alpha1.HelmRelease) (helmManager helmrelease.Manager,
	helmReleaseSecret *corev1.Secret,
	secretCreated bool,
	err error) {
	configMap, err := utils.GetConfigMap(r.client, s.Namespace, s.Spec.ConfigMapRef)
	if err != nil {
		klog.Error(err)
		return nil, helmReleaseSecret, false, err
	}

	secret, err := utils.GetSecret(r.client, s.Namespace, s.Spec.SecretRef)
	if err != nil {
		klog.Error(err, " - Failed to retrieve secret ", s.Spec.SecretRef.Name)
		return nil, helmReleaseSecret, secretCreated, err
	}

	mgr, err := manager.New(r.config, manager.Options{
		Namespace: s.GetNamespace(),
		//Disable MetricsListener
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Error(err, " - Failed to create a new manager.")
		return nil, helmReleaseSecret, secretCreated, err
	}

	chartsDir := os.Getenv(appv1alpha1.ChartsDir)
	if chartsDir == "" {
		chartsDir, err = ioutil.TempDir("/tmp", "charts")
		if err != nil {
			klog.Error(err, " - Can not create tempdir")
			return nil, helmReleaseSecret, secretCreated, err
		}
	}

	chartDir, err := utils.DownloadChart(configMap, secret, chartsDir, s)
	klog.V(3).Info("ChartDir: ", chartDir)

	if s.DeletionTimestamp.IsZero() {
		if err != nil {
			klog.Error(err, " - Failed to download the chart")
			return nil, helmReleaseSecret, secretCreated, err
		}
	} else if err != nil {
		//If error when download for deletion then create a fake chart.yaml.
		//The helmrelease manager needs only the releaseName
		klog.Info("Unable to download ChartDir: ", chartDir, " creating a fake chart.yaml")
		chartDir, err = utils.CreateFakeChart(chartsDir, s)
		if err != nil {
			klog.Error(err, " - Failed to create fake chart for uninstall")
			return nil, helmReleaseSecret, secretCreated, err
		}
	}

	f := helmrelease.NewManagerFactory(mgr, chartDir)

	helmReleaseSecret, err = utils.GetSecret(r.client,
		s.Namespace,
		&corev1.ObjectReference{Name: s.Spec.ReleaseName})
	//Secret already exists
	if err == nil {
		//Secret not owned by the helmrelease and we have a duplicate name
		if !utils.IsOwned(s.ObjectMeta, helmReleaseSecret.ObjectMeta) {
			return nil, nil, secretCreated,
				fmt.Errorf("duplicate release name: found existing release with name %q for another helmRelease %v",
					s.Spec.ReleaseName, helmReleaseSecret.GetOwnerReferences())
		}
	} else if errors.IsNotFound(err) {
		//Secret not found
		//Create a new secret for this helmrelease
		helmReleaseSecret, err = createSecret(r, s)
		if err != nil {
			klog.Error(err)
			return nil, helmReleaseSecret, secretCreated, err
		}
		secretCreated = true
	} else {
		//Another type of error
		return nil, nil, secretCreated, err
	}

	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(helmReleaseSecret.GroupVersionKind())
	o.SetNamespace(helmReleaseSecret.GetNamespace())

	o.SetName(helmReleaseSecret.GetName())
	klog.V(2).Info("ReleaseName :", o.GetName())
	o.SetUID(helmReleaseSecret.GetUID())
	klog.V(5).Info("uuid:", o.GetUID())

	if s.Spec.Values != "" {
		var spec interface{}

		err = yaml.Unmarshal([]byte(s.Spec.Values), &spec)
		if err != nil {
			klog.Error(err, " - Failed to Unmarshal the values ", s.Spec.Values)
			return nil, helmReleaseSecret, secretCreated, err
		}

		o.Object["spec"] = spec
	}

	helmManager, err = f.NewManager(o)
	if err != nil {
		_ = deleteCreatedSecret(r, helmReleaseSecret, secretCreated)
	}

	return helmManager, helmReleaseSecret, secretCreated, err
}

func createSecret(
	r *ReconcileHelmRelease,
	s *appv1alpha1.HelmRelease) (releaseSecret *corev1.Secret, err error) {
	//Check if the helmrelease has an annotation
	if releaseSecretAnnotation, ok := s.GetAnnotations()[appv1alpha1.ReleaseSecretAnnotationKey]; ok {
		//Search namespace and name of the secret
		releaseSecretsAnnotation := strings.Split(releaseSecretAnnotation, "/")
		if len(releaseSecretsAnnotation) != 2 {
			err = fmt.Errorf("invalid release-secret annotation %s", releaseSecretAnnotation)
			klog.Error(err)

			return nil, err
		}

		//Check if the secret name change. The secret name is the release name of the helmchart.
		if releaseSecretsAnnotation[0] != s.GetNamespace() || releaseSecretsAnnotation[1] != s.Spec.ReleaseName {
			err = fmt.Errorf("release name can not be changed: new %s, old %s",
				s.Spec.ReleaseName, releaseSecretsAnnotation[1])
			klog.Error(err)

			return nil, err
		}
	}

	//Create the secret with the release name as name and
	//the helmrelease as owner
	releaseSecret = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
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

	err = r.client.Create(context.TODO(), releaseSecret)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	//Re-add the TypeMeta
	releaseSecret.TypeMeta = metav1.TypeMeta{
		Kind:       "Secret",
		APIVersion: "v1",
	}

	return releaseSecret, nil
}

func deleteCreatedSecret(r *ReconcileHelmRelease, secret *corev1.Secret, secretCreated bool) error {
	if secretCreated {
		return deleteSecret(r, secret)
	}

	return nil
}

func deleteSecret(r *ReconcileHelmRelease, secret *corev1.Secret) error {
	if secret != nil {
		err := r.client.Delete(context.TODO(), secret)
		if err != nil {
			klog.Error(err, " - Failed to delete release secret")
			return err
		}
	}

	return nil
}
