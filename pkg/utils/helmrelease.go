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
	"fmt"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

const (
	releaseFinalizer = "app.ibm.com/helmrelease"
)

func HasFinalizer(h *appv1alpha1.HelmRelease) bool {
	currentFinalizers := h.ObjectMeta.Finalizers
	for _, f := range currentFinalizers {
		if f == releaseFinalizer {
			return true
		}
	}

	return false
}

func AddFinalizer(helmObj *appv1alpha1.HelmRelease) {
	helmObj.ObjectMeta.Finalizers = append(helmObj.ObjectMeta.Finalizers, releaseFinalizer)
}

func RemoveFinalizer(helmObj *appv1alpha1.HelmRelease) {
	newSlice, _ := remove(releaseFinalizer, helmObj.ObjectMeta.Finalizers)
	if len(newSlice) == 0 {
		newSlice = nil
	}

	helmObj.ObjectMeta.Finalizers = newSlice
}

// remove item from slice without keeping order
func remove(item string, s []string) ([]string, error) {
	index := findIndex(item, s)
	if index == -1 {
		return []string{}, fmt.Errorf("%s not present in %v", item, s)
	}

	for index != -1 {
		index := findIndex(item, s)
		if index == -1 {
			break
		}

		s[index] = s[len(s)-1]
		s[len(s)-1] = ""
		s = s[:len(s)-1]
	}

	return s, nil
}

func findIndex(target string, s []string) int {
	for i := range s {
		if s[i] == target {
			return i
		}
	}

	return -1
}
