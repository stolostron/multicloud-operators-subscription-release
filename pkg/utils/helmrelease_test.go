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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

func TestHasFinalizer(t *testing.T) {
	hr := &appv1alpha1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "subscription-release-test-1-cr",
			Namespace:  "default",
			Finalizers: []string{releaseFinalizer},
		},
	}

	assert.Equal(t, true, HasFinalizer(hr))

	RemoveFinalizer(hr)

	assert.Equal(t, false, HasFinalizer(hr))

	AddFinalizer(hr)

	assert.Equal(t, true, HasFinalizer(hr))
}

func TestGenerateHelmReleaseName(t *testing.T) {
	base := "123456789"
	timestamp := metav1.NewTime(time.Now())
	name1 := GenerateHelmReleaseName(base, timestamp)
	assert.Equal(t, name1, base)

	name2 := GenerateHelmReleaseName(base, timestamp)
	assert.Equal(t, name1, name2)

	base = "1234567890123456789012345678901234567890123456789012345678901234"
	name1 = GenerateHelmReleaseName(base, timestamp)
	assert.Equal(t, true, len(name1) <= maxNameLength)
	assert.Equal(t, true, strings.HasPrefix(name1, base[:maxGeneratedNameLength]))

	base = "1234567890123456789012345678901234567890123456789012345678"
	name1 = GenerateHelmReleaseName(base, timestamp)
	assert.Equal(t, true, len(name1) <= maxNameLength)
	assert.Equal(t, base, name1)
}
