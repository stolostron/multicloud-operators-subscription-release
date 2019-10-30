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
	"testing"
	"time"

	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

var c client.Client

const timeout = time.Second * 5

var (
	helmReleaseName = "example-helmrelease"
	helmReleaseNS   = "kube-system"
	helmReleaseKey  = types.NamespacedName{
		Name:      helmReleaseName,
		Namespace: helmReleaseNS,
	}
)

func TestReconcileGitHubSuccess(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

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

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	instanceResp := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseSuccess))

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), &client.ListOptions{}, helmReleaseList)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	for _, hr := range helmReleaseList.Items {
		err = c.Delete(context.TODO(), &hr)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}
}

func TestReconcileGitHubFailure(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

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
					ChartPath: "wrong path",
				},
			},
			ReleaseName: "subscription-release-test-1",
			ChartName:   "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	instanceResp := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseFailed))

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), &client.ListOptions{}, helmReleaseList)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	for _, hr := range helmReleaseList.Items {
		err = c.Delete(context.TODO(), &hr)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}
}

func TestReconcileHelmRepoSuccess(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

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
				SourceType: appv1alpha1.HelmRepoSourceType,
				HelmRepo: &appv1alpha1.HelmRepo{
					Urls: []string{"https://raw.github.com/IBM/multicloud-operators-subscription-release/master/test/helmrepo/subscription-release-test-1-0.1.0.tgz"},
				},
			},
			ChartName:   "subscription-release-test-1",
			ReleaseName: "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	instanceResp := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	t.Logf("instanceResp: %v", instanceResp)
	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseSuccess))

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), &client.ListOptions{}, helmReleaseList)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	for _, hr := range helmReleaseList.Items {
		err = c.Delete(context.TODO(), &hr)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}
}

func TestReconcileHelmRepoFailure(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

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
				SourceType: appv1alpha1.HelmRepoSourceType,
				HelmRepo: &appv1alpha1.HelmRepo{
					Urls: []string{"https://raw.github.com/IBM/multicloud-operators-subscription-release/wrongurl"},
				},
			},
			ChartName:   "subscription-release-test-1",
			ReleaseName: "subscription-release-test-1",
		},
	}

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: helmReleaseKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	instanceResp := &appv1alpha1.HelmRelease{}
	err = c.Get(context.TODO(), helmReleaseKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmReleaseFailed))

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), &client.ListOptions{}, helmReleaseList)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	for _, hr := range helmReleaseList.Items {
		err = c.Delete(context.TODO(), &hr)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}
}
