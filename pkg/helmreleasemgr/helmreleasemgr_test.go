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
	"context"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

var (
	helmReleaseNS = "kube-system"
)

func TestNewManager(t *testing.T) {
	helmReleaseName := "test-new-manager"
	g := gomega.NewGomegaWithT(t)

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	assert.NoError(t, err)

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &appv1alpha1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmReleaseName,
			Namespace: helmReleaseNS,
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
		},
	}

	c := mgr.GetClient()

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, err = NewHelmReleaseManager(mgr.GetConfig(), nil, nil, instance)
	assert.NoError(t, err)
}

func TestNewManagerShortReleaseName(t *testing.T) {
	helmReleaseName := "test-new-manager-short-release-name"
	g := gomega.NewGomegaWithT(t)

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	assert.NoError(t, err)

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &appv1alpha1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmReleaseName,
			Namespace: helmReleaseNS,
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.GitHubSourceType,
				GitHub: &appv1alpha1.GitHub{
					Urls:      []string{"https://github.com/IBM/multicloud-operators-subscription-release.git"},
					ChartPath: "test/github/subscription-release-test-1",
				},
			},
			ReleaseName: "sub",
			ChartName:   "subscription-release-test-1",
		},
	}

	c := mgr.GetClient()

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, err = NewHelmReleaseManager(mgr.GetConfig(), nil, nil, instance)
	assert.NoError(t, err)
}

func TestNewManagerValues(t *testing.T) {
	helmReleaseName := "test-new-manager-values"
	g := gomega.NewGomegaWithT(t)

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	assert.NoError(t, err)

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &appv1alpha1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmReleaseName,
			Namespace: helmReleaseNS,
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.GitHubSourceType,
				GitHub: &appv1alpha1.GitHub{
					Urls:      []string{"https://github.com/IBM/multicloud-operators-subscription-release.git"},
					ChartPath: "test/github/subscription-release-test-1",
				},
			},
			ReleaseName: "sub",
			ChartName:   "subscription-release-test-1",
			Values:      "l1:v1",
		},
	}
	c := mgr.GetClient()

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	//Values well formed
	_, err = NewHelmReleaseManager(mgr.GetConfig(), nil, nil, instance)
	assert.NoError(t, err)
	//Values not a yaml
	instance.Spec.Values = "l1:\nl2"
	_, err = NewHelmReleaseManager(mgr.GetConfig(), nil, nil, instance)
	assert.Error(t, err)
}

func TestNewManagerErrors(t *testing.T) {
	helmReleaseName := "test-new-manager-errors"
	g := gomega.NewGomegaWithT(t)

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	assert.NoError(t, err)

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &appv1alpha1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmReleaseName,
			Namespace: helmReleaseNS,
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.GitHubSourceType,
				GitHub: &appv1alpha1.GitHub{
					Urls:      []string{"https://github.com/IBM/multicloud-operators-subscription-release.git"},
					ChartPath: "test/github/subscription-release-test-1",
				},
			},
			ReleaseName: "sub",
			ChartName:   "subscription-release-test-1",
		},
	}
	c := mgr.GetClient()

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	//Config nil
	_, err = NewHelmReleaseManager(nil, nil, nil, instance)
	assert.Error(t, err)
	//Download Chart should fail
	instance.Spec.Source.GitHub.Urls[0] = "wrongurl"
	instance.Spec.Values = "l1:\nl2"
	_, err = NewHelmReleaseManager(mgr.GetConfig(), nil, nil, instance)
	assert.Error(t, err)
}
