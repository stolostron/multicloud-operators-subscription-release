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

package helmchartsubscription

import (
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

var c client.Client

const timeout = time.Second * 5

var (
	helmChartSubscriptionName = "example-helmchartsubscription"
	helmChartSubscriptionNS   = "kube-system"
	helmChartSubscriptionKey  = types.NamespacedName{
		Name:      helmChartSubscriptionName,
		Namespace: helmChartSubscriptionNS,
	}
)

const sub1 = `apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  annotations:
    tillerVersion: 2.4.0
  name: test
  namespace: default
spec:
  channel: default/ope
  installPlanApproval: Manual
  name: "subscription-release-test-1"
  chartsSource: 
    type: helmrepo
    helmRepo:
      urls:
      - https://raw.github.com/IBM/multicloud-operators-subscription-release/master/test/helmrepo
  packageFilter:
    keywords:
    - MCM
    version: ">0.1.0"
  packageOverrides:
  - packageName: subscription-release-test-1
    packageOverrides:
    - path: spec.values
      value: |
        att1: hello`

func TestReconcileHelmRepoSuccess(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})

	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()
	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &appv1alpha1.HelmChartSubscription{}
	err = yaml.Unmarshal([]byte(sub1), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	instance.Name = helmChartSubscriptionName
	instance.Namespace = helmChartSubscriptionNS

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: helmChartSubscriptionKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(5 * time.Second)

	instanceResp := &appv1alpha1.HelmChartSubscription{}
	err = c.Get(context.TODO(), helmChartSubscriptionKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmChartSubscriptionSuccess))

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), helmReleaseList, &client.ListOptions{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(1).To(gomega.Equal(len(helmReleaseList.Items)))

	for _, hr := range helmReleaseList.Items {
		err = c.Delete(context.TODO(), &hr)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}

	helmChartSubscription := &appv1alpha1.HelmChartSubscriptionList{}
	err = c.List(context.TODO(), helmChartSubscription, &client.ListOptions{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	for _, hcs := range helmChartSubscription.Items {
		err = c.Delete(context.TODO(), &hcs)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}
}

func TestReconcileHelmRepoSuccessFilter(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})

	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &appv1alpha1.HelmChartSubscription{}
	err = yaml.Unmarshal([]byte(sub1), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	instance.Name = helmChartSubscriptionName
	instance.Namespace = helmChartSubscriptionNS
	instance.Spec.PackageFilter.Version = "0.3.0"

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: helmChartSubscriptionKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(5 * time.Second)

	instanceResp := &appv1alpha1.HelmChartSubscription{}
	err = c.Get(context.TODO(), helmChartSubscriptionKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmChartSubscriptionSuccess))

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), helmReleaseList, &client.ListOptions{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(0).To(gomega.Equal(len(helmReleaseList.Items)))

	helmChartSubscription := &appv1alpha1.HelmChartSubscriptionList{}
	err = c.List(context.TODO(), helmChartSubscription, &client.ListOptions{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	for _, hcs := range helmChartSubscription.Items {
		err = c.Delete(context.TODO(), &hcs)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}
}

func TestReconcileHelmRepoFailed(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})

	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &appv1alpha1.HelmChartSubscription{}
	err = yaml.Unmarshal([]byte(sub1), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	instance.Name = helmChartSubscriptionName
	instance.Namespace = helmChartSubscriptionNS
	instance.Spec.Source.HelmRepo.Urls[0] = "https://raw.github.com/IBM/multicloud-operators-subscription-release/wrongurl"

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: helmChartSubscriptionKey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(5 * time.Second)

	instanceResp := &appv1alpha1.HelmChartSubscription{}
	err = c.Get(context.TODO(), helmChartSubscriptionKey, instanceResp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(instanceResp.Status.Status).To(gomega.Equal(appv1alpha1.HelmChartSubscriptionFailed))

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), helmReleaseList, &client.ListOptions{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(0).To(gomega.Equal(len(helmReleaseList.Items)))

	helmChartSubscription := &appv1alpha1.HelmChartSubscriptionList{}
	err = c.List(context.TODO(), helmChartSubscription, &client.ListOptions{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	for _, hcs := range helmChartSubscription.Items {
		err = c.Delete(context.TODO(), &hcs)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}
}
