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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LabelsChecker checks labels against a labelSelector
func LabelsChecker(labelSelector *metav1.LabelSelector, ls map[string]string) bool {
	clSelector, err := ConvertLabels(labelSelector)
	if err != nil {
		klog.Error("Failed to set label selector: ", labelSelector, " err:", err)
	}

	return clSelector.Matches(labels.Set(ls))
}

// ConvertLabels converts label selector to lables.Selector
func ConvertLabels(labelSelector *metav1.LabelSelector) (labels.Selector, error) {
	if labelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(labelSelector)

		if err != nil {
			return labels.Nothing(), err
		}

		return selector, nil
	}

	return labels.Everything(), nil
}

//GetAccessToken retrieve the accessToken
func GetAccessToken(secret *corev1.Secret) string {
	if accessToken, ok := secret.Data["accessToken"]; ok {
		return string(accessToken)
	}

	return ""
}

//GetPassword retrieve the password
func GetPassword(secret *corev1.Secret) string {
	if password, ok := secret.Data["password"]; ok {
		return string(password)
	}

	return ""
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
