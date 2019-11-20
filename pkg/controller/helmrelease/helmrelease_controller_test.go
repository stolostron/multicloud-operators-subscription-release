// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helmrelease

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

// var c client.Client

const timeout = time.Second * 5

var (
	helmReleaseNS = "kube-system"
)

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	t.Log("Create manager")

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
		LeaderElection:     false,
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c := mgr.GetClient()

	rec := &ReconcileHelmRelease{
		config: mgr.GetConfig(),
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}

	t.Log("Setup test reconcile")

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	t.Log("Start test reconcile")

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	//
	//Github succeed
	//
	t.Log("Github succeed test")

	helmReleaseName := "example-github-succeed"
	helmReleaseKey := types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
	instance := &appv1alpha1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "app.ibm.com/v1alpha1",
		},
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
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(4 * time.Second)

	instanceResp := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseSuccess))

	//
	//Github failed
	//
	t.Log("Github failed test")

	helmReleaseName = "example-github-failed"
	helmReleaseKey = types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
	instance = &appv1alpha1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "app.ibm.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmReleaseName,
			Namespace: helmReleaseNS,
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.GitHubSourceType,
				GitHub: &appv1alpha1.GitHub{
					Urls:      []string{"https://github.com/IBM/multicloud-operators-subscription-release.git"},
					ChartPath: "wrong path",
				},
			},
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	instanceResp = &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseFailed))

	//
	//helmRepo succeeds
	//
	t.Log("helmrepo succeed test")

	helmReleaseName = "example-helmrepo-succeed"
	helmReleaseKey = types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
	instance = &appv1alpha1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "app.ibm.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmReleaseName,
			Namespace: helmReleaseNS,
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.HelmRepoSourceType,
				HelmRepo: &appv1alpha1.HelmRepo{
					Urls: []string{"https://raw.github.com/IBM/multicloud-operators-subscription-release/master/test/helmrepo/subscription-release-test-1-0.1.0.tgz"},
				},
			},
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	instanceResp = &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseSuccess))

	secret := &corev1.Secret{}
	err = c.Get(context.TODO(), types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}, secret)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	//Check duplicate
	helmReleaseName = "example-helmrepo-succeed-duplicate"
	helmReleaseKey = types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
	instance.ObjectMeta = metav1.ObjectMeta{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	instanceResp = &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseFailed))

	//
	//helmRepo failure
	//
	t.Log("Github failure test")

	helmReleaseName = "example-helmrepo-failure"
	helmReleaseKey = types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
	instance = &appv1alpha1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "app.ibm.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmReleaseName,
			Namespace: helmReleaseNS,
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.HelmRepoSourceType,
				HelmRepo: &appv1alpha1.HelmRepo{
					Urls: []string{"https://raw.github.com/IBM/multicloud-operators-subscription-release/wrongurl"},
				},
			},
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	instanceResp = &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseFailed))

	//
	//Github succeed create-delete
	//
	t.Log("Github succeed create-delete test")

	helmReleaseName = "example-github-delete"
	helmReleaseKey = types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
	instance = &appv1alpha1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "app.ibm.com/v1alpha1",
		},
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
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	//Creation
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(reconcile.Request{NamespacedName: helmReleaseKey})))

	time.Sleep(4 * time.Second)

	instanceRespCD := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceRespCD)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	time.Sleep(2 * time.Second)

	g.Expect(instanceRespCD.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseSuccess))

	//Deletion
	err = c.Delete(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	time.Sleep(2 * time.Second)

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(reconcile.Request{NamespacedName: helmReleaseKey})))

	time.Sleep(2 * time.Second)

	instanceRespDel := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceRespDel)
	g.Expect(err).To(gomega.HaveOccurred())

	time.Sleep(2 * time.Second)

	//
	//Github succeed create-update
	//
	t.Log("Github succeed create-update")

	helmReleaseName = "example-github-update"
	helmReleaseKey = types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
	instance = &appv1alpha1.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: "app.ibm.com/v1alpha1",
		},
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
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	//Creation
	t.Log("Github succeed create-update -> CR create")

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(reconcile.Request{NamespacedName: helmReleaseKey})))

	time.Sleep(4 * time.Second)

	t.Log("Github succeed create-update -> CR get response")

	instanceRespCU := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceRespCU)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	time.Sleep(2 * time.Second)

	g.Expect(instanceRespCU.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseSuccess))

	//Update
	t.Log("Github succeed create-update -> CR get")

	err = c.Get(context.TODO(), helmReleaseKey, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	instance.Spec.Values = "l1:v1"

	t.Log("Github succeed create-update -> CR update")

	err = c.Update(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	time.Sleep(2 * time.Second)

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(reconcile.Request{NamespacedName: helmReleaseKey})))

	time.Sleep(2 * time.Second)

	t.Log("Github succeed create-update -> CR get response")

	instanceRespUp := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceRespUp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// TestNewManager
	helmReleaseName = "test-new-manager"

	instance = &appv1alpha1.HelmRelease{
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

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, err = newHelmReleaseManager(rec, instance)
	assert.NoError(t, err)

	// TestNewManagerShortReleaseName
	helmReleaseName = "test-new-manager-short-release-name"
	instance = &appv1alpha1.HelmRelease{
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
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(4 * time.Second)

	_, err = newHelmReleaseManager(rec, instance)
	assert.NoError(t, err)

	// TestNewManagerValues
	helmReleaseName = "test-new-manager-values"
	instance = &appv1alpha1.HelmRelease{
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
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
			Values:      "l1:v1",
		},
	}

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	//Values well formed
	_, err = newHelmReleaseManager(rec, instance)
	assert.NoError(t, err)
	//Values not a yaml
	instance.Spec.Values = "l1:\nl2"
	_, err = newHelmReleaseManager(rec, instance)
	assert.Error(t, err)

	// TestNewManagerErrors
	helmReleaseName = "test-new-manager-errors"

	instance = &appv1alpha1.HelmRelease{
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
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	//Config nil
	rec.config = nil
	_, err = newHelmReleaseManager(rec, instance)
	assert.Error(t, err)
	//Download Chart should fail
	rec.config = mgr.GetConfig()
	instance.Spec.Source.GitHub.Urls[0] = "wrongurl"
	instance.Spec.Values = "l1:\nl2"
	_, err = newHelmReleaseManager(rec, instance)
	assert.Error(t, err)

	// TestNewManagerForDeletion
	chartsDir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(chartsDir)

	err = os.Setenv(appv1alpha1.ChartsDir, chartsDir)
	assert.NoError(t, err)

	helmReleaseName = "test-new-manager-delete"

	instance = &appv1alpha1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helmReleaseName,
			Namespace: helmReleaseNS,
		},
		Spec: appv1alpha1.HelmReleaseSpec{
			Source: &appv1alpha1.Source{
				SourceType: appv1alpha1.GitHubSourceType,
				GitHub: &appv1alpha1.GitHub{
					Urls:      []string{"https://github.com/IBM/wrongurl"},
					ChartPath: "test/github/subscription-release-test-1",
				},
			},
			ReleaseName: helmReleaseName,
			ChartName:   "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	instance.GetObjectMeta().SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
	mgrhr, err := newHelmReleaseManager(rec, instance)
	assert.NoError(t, err)

	assert.Equal(t, mgrhr.ReleaseName(), helmReleaseName)

	if _, err := os.Stat(filepath.Join(chartsDir, instance.Spec.ChartName, "Chart.yaml")); err != nil {
		assert.Fail(t, err.Error())
	}
}
