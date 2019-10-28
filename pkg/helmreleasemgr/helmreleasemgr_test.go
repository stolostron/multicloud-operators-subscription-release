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
	"testing"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewManager(t *testing.T) {
	hr := &appv1alpha1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.GitHubSourceType,
				GitHub: &appv1alpha1.GitHub{
					Urls:      []string{"https://github.com/IBM/multicloud-operators-subscription-release.git"},
					ChartPath: "test/github/subscription-release-test-1",
				},
			},
			ReleaseName: "subscription-release-test-1",
			ChartName:   "subscription-release-test-1",
			Values:      "att1: hello",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	os.Setenv(appv1alpha1.ChartsDir, dir)

	mgr, err := NewManager(nil, nil, hr)
	assert.NoError(t, err)

	assert.Equal(t, "subs-", mgr.ReleaseName())
}
