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

package helmreposubscriber

import (
	"context"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	appv1alpha1 "github.com/IBM/multicloud-operators-subscription-release/pkg/apis/app/v1alpha1"
)

var c client.Client

const index = `apiVersion: v1
entries:
  ibm-cfee-installer:
  - created: 2019-06-25T17:38:32.128540891Z
    description: Cloud Foundry Enterprise Environment deployment tool
    digest: c3a86622131b877863bf292a2c991a66a5c1132a2db3297eea8c423cd001ab24
    icon: https://www.ibm.com/cloud-computing/images/new-cloud/img/cloud.png
    keywords:
    - amd64
    - DevOps
    - Development
    - ICP
    name: ibm-cfee-installer
    tillerVersion: '>=2.4.0'
    urls:
    - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-cfee-installer-3.2.0-60-481af98.20190501110704.tgz
    version: 3.2.0-alpha
  - created: 2019-06-25T17:38:32.21333478Z
    description: Cloud Foundry Enterprise Environment deployment tool
    digest: cd7d7e7c109dae92c7a92896efe924a43829b395d37f9ec2cfb481d66b9b14d0
    icon: https://www.ibm.com/cloud-computing/images/new-cloud/img/cloud.png
    keywords:
    - amd64
    - DevOps
    - Development
    - ICP
    name: ibm-cfee-installer
    tillerVersion: '>=2.4.0'
    urls:
    - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-cfee-installer-3.2.0-62.tgz
    version: 3.2.0-beta
  ibm-mcm-prod:
  - apiVersion: v1
    appVersion: "1.0"
    created: 2019-06-25T17:38:32.41778815Z
    description: IBM Multicloud Manager
    digest: 1b5038b4380a388ac30cfcd057519a6827e0100a09f983687ec80d985fda8860
    keywords:
    - Analytics
    - deploy
    - Commercial
    - amd64
    name: ibm-mcm-prod
    tillerVersion: '>=2.7.3'
    urls:
    - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-mcm-prod-3.1.2.tgz
    version: 3.1.2
  ibm-mcmk-prod:
  - apiVersion: v1
    appVersion: "1.0"
    created: 2019-06-25T17:38:32.511798404Z
    description: IBM Multicloud Manager Klusterlet
    digest: e80c01b33b83b3b16da7ee67434ed672b3b6483d2bc0c1f1f2171bfe449ed84b
    keywords:
    - ICP
    - Analytics
    - deploy
    - Commercial
    - amd64
    - ppc64le
    name: ibm-mcmk-prod
    tillerVersion: '>=2.7.3'
    urls:
    - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-mcmk-prod-3.1.2.tgz
    version: 3.1.3
`

const sub1 = `apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  annotations:
    tillerVersion: 2.4.0
  name: test-helmsubscriber
  namespace: default
spec:
  channel: default/ope
  autoUpgrade: true
  name: "ibm-cfee-installer"
  chartsSource: 
    type: helmrepo
    helmRepo:
      urls:
      - https://mycluster.icp:8443/helm-repo/charts
  packageFilter:
    keywords:
    - ICP
    annotations:
      tillerVersion: 2.4.0
    version: ">0.2.2"
  configRef:
    name: mycluster-config
  packageOverrides:
  - packageName: ibm-cfee-installer
    packageOverrides:
    - path: spec.values
      value: |
        att1: hello`

const helmRepoSub = `apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  annotations:
    tillerVersion: 2.4.0
  name: test-helmsubscriber
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

const gitRepoSub = `apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  annotations:
    tillerVersion: 2.4.0
  name: test-helmsubscriber
  namespace: default
spec:
  channel: default/ope
  installPlanApproval: Manual
  name: "subscription-release-test-1"
  chartsSource:
    type: github
    github:
      urls:
      - https://github.com/IBM/multicloud-operators-subscription-release.git
      chartsPath: test/github
  packageFilter:
    version: "0.1.0"
  packageOverrides:
  - packageName: subscription-release-test-1
    packageOverrides:
    - path: spec.values
      value: |
        att1: hello`

func TestRestart(t *testing.T) {
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

	subscription := &appv1alpha1.HelmChartSubscription{}
	err = yaml.Unmarshal([]byte(gitRepoSub), subscription)
	assert.NoError(t, err)

	subscription.Spec.InstallPlanApproval = appv1alpha1.ApprovalAutomatic

	c = mgr.GetClient()

	err = c.Create(context.TODO(), subscription)
	assert.NoError(t, err)

	subscriber := &HelmRepoSubscriber{
		Client:                c,
		Scheme:                mgr.GetScheme(),
		HelmChartSubscription: subscription,
	}

	//Start subscriber
	err = subscriber.Restart()
	assert.NoError(t, err)

	assert.Equal(t, true, subscriber.started)

	time.Sleep(2 * time.Second)

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), helmReleaseList, &client.ListOptions{})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(helmReleaseList.Items))

	//Update subscriber
	err = subscriber.Update(subscription)
	assert.NoError(t, err)

	assert.Equal(t, true, subscriber.started)

	//Stop subscriber

	err = subscriber.Stop()
	assert.NoError(t, err)

	assert.Equal(t, false, subscriber.started)

	subscription.Spec.InstallPlanApproval = appv1alpha1.ApprovalManual

	//Start subscriber in Manual mode
	err = subscriber.Restart()
	assert.NoError(t, err)

	assert.Equal(t, false, subscriber.started)

	//Update subscriber in Manual mode
	err = subscriber.Update(subscription)
	assert.NoError(t, err)

	assert.Equal(t, false, subscriber.started)

	//Stop subscriber in Manual mode

	err = subscriber.Stop()
	assert.NoError(t, err)

	assert.Equal(t, false, subscriber.started)

	for _, hr := range helmReleaseList.Items {
		err = c.Delete(context.TODO(), &hr)
		assert.NoError(t, err)
	}

	subscriptionList := &appv1alpha1.HelmChartSubscriptionList{}
	err = c.List(context.TODO(), subscriptionList, &client.ListOptions{})
	assert.NoError(t, err)

	for _, s := range subscriptionList.Items {
		err = c.Delete(context.TODO(), &s)
		assert.NoError(t, err)
	}
}

func TestDoHelmChartSubscription(t *testing.T) {
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

	subscription := &appv1alpha1.HelmChartSubscription{}
	err = yaml.Unmarshal([]byte(gitRepoSub), subscription)
	assert.NoError(t, err)

	c = mgr.GetClient()

	err = c.Create(context.TODO(), subscription)
	assert.NoError(t, err)

	subscriber := &HelmRepoSubscriber{
		Client:                c,
		Scheme:                mgr.GetScheme(),
		HelmChartSubscription: subscription,
	}

	err = subscriber.doHelmChartSubscription()
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	helmReleaseList := &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), helmReleaseList, &client.ListOptions{})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(helmReleaseList.Items))

	//Rerun for update, no new helmRelease must be created because hash didn't change
	err = subscriber.doHelmChartSubscription()
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	helmReleaseList = &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), helmReleaseList, &client.ListOptions{})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(helmReleaseList.Items))

	//Rerun for update, no new helmRelease must be created because already exist and Spec identical
	subscriber.HelmRepoHash = ""
	err = subscriber.doHelmChartSubscription()
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	helmReleaseList = &appv1alpha1.HelmReleaseList{}
	err = c.List(context.TODO(), helmReleaseList, &client.ListOptions{})
	assert.NoError(t, err)

	assert.Equal(t, 1, len(helmReleaseList.Items))

	for _, hr := range helmReleaseList.Items {
		err = c.Delete(context.TODO(), &hr)
		assert.NoError(t, err)
	}

	subscriptionList := &appv1alpha1.HelmChartSubscriptionList{}
	err = c.List(context.TODO(), subscriptionList, &client.ListOptions{})
	assert.NoError(t, err)

	for _, s := range subscriptionList.Items {
		err = c.Delete(context.TODO(), &s)
		assert.NoError(t, err)
	}
}

func TestLoadIndex(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 2, len(chartVersions))

	name := chartVersions[1].GetName()
	assert.Equal(t, "ibm-cfee-installer", name)
}

func Test_MatchingNameCharts(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{
				Package: "ibm-cfee-installer",
			},
		},
	}
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(indexFile.Entries))

	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingWithoutPackageName(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{},
		},
	}
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(indexFile.Entries))

	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingWithoutOPSubscriptionSpec(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{},
		},
	}
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(indexFile.Entries))

	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingWithoutSubscriptionSpec(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{},
	}
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(indexFile.Entries))

	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingDigest(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					Annotations: map[string]string{
						"digest": "c3a86622131b877863bf292a2c991a66a5c1132a2db3297eea8c423cd001ab24",
					},
				},
			},
		},
	}
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(indexFile.Entries))

	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingTillerVersion(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					Annotations: map[string]string{
						"tillerVersion": "2.4.0",
					},
				},
			},
		},
	}
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(indexFile.Entries))

	versionedCharts := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(versionedCharts))
}

func Test_MatchingTillerVersionNotFound(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					Annotations: map[string]string{
						"tillerVersion": "2.2.0",
					},
				},
			},
		},
	}

	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(indexFile.Entries))
}

func Test_MatchingVersion(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					Version: ">=3.1.3 <=3.2.0-alpha",
				},
			},
		},
	}
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(indexFile.Entries))

	versionedCharts := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(versionedCharts))
	assert.Equal(t, "3.2.0-alpha", versionedCharts[0].GetVersion())

	versionedChartsNil := indexFile.Entries["ibm-mcm-prod"]
	assert.Nil(t, versionedChartsNil)

	versionedCharts = indexFile.Entries["ibm-mcmk-prod"]
	assert.Equal(t, 1, len(versionedCharts))
}

func Test_CheckKeywords(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"ICP": "true",
						},
					},
				},
			},
		},
	}
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(indexFile.Entries))

	indexFile, err = LoadIndex([]byte(index))
	assert.NoError(t, err)

	s.HelmChartSubscription.Spec.PackageFilter = nil
	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(indexFile.Entries))
}

func Test_takeLatestVersion(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{}

	err = s.takeLatestVersion(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(indexFile.Entries))

	versionedCharts := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(versionedCharts))

	versrionedChart := versionedCharts[0]
	assert.Equal(t, "3.2.0-beta", versrionedChart.GetVersion())

	versionedCharts = indexFile.Entries["ibm-mcm-prod"]
	assert.Equal(t, 1, len(versionedCharts))

	versrionedChart = versionedCharts[0]
	assert.Equal(t, "3.1.2", versrionedChart.GetVersion())

	versionedCharts = indexFile.Entries["ibm-mcmk-prod"]
	assert.Equal(t, 1, len(versionedCharts))

	versrionedChart = versionedCharts[0]
	assert.Equal(t, "3.1.3", versrionedChart.GetVersion())
}

func Test_filterCharts(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{
				Package: "ibm-mcm-prod",
				PackageFilter: &appv1alpha1.PackageFilter{
					Version: ">=3.1.2 <3.2.0",
					Annotations: map[string]string{
						"tillerVersion": "2.4.0",
					},
				},
			},
		},
	}

	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	//Zero because no version for tiller >=2.7.3
	assert.Equal(t, 0, len(indexFile.Entries))
}

func Test_filterChartsLatest(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	s := &HelmRepoSubscriber{
		HelmChartSubscription: &appv1alpha1.HelmChartSubscription{
			Spec: appv1alpha1.HelmChartSubscriptionSpec{
				Package: "ibm-cfee-installer",
			},
		},
	}

	err = s.filterCharts(indexFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(indexFile.Entries))

	versionedCharts := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(versionedCharts))

	versrionedChart := versionedCharts[0]
	assert.Equal(t, "3.2.0-beta", versrionedChart.GetVersion())
}

func TestNewHelmChartHelmReleaseForCR(t *testing.T) {
	s := &appv1alpha1.HelmChartSubscription{}
	err := yaml.Unmarshal([]byte(sub1), s)
	assert.NoError(t, err)

	subscriber := &HelmRepoSubscriber{
		HelmChartSubscription: s,
	}

	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	hr, err := subscriber.newHelmChartHelmReleaseForCR(indexFile.Entries["ibm-cfee-installer"][0])
	assert.NoError(t, err)
	assert.Equal(t, "ibm-cfee-installer-test-helmsubscriber-default", hr.Spec.ReleaseName)
}

func TestGetValues(t *testing.T) {
	s := &appv1alpha1.HelmChartSubscription{}
	err := yaml.Unmarshal([]byte(sub1), s)
	assert.NoError(t, err)

	subscriber := &HelmRepoSubscriber{
		HelmChartSubscription: s,
	}

	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)

	values, err := subscriber.getValues(indexFile.Entries["ibm-cfee-installer"][0])
	assert.NoError(t, err)

	assert.Equal(t, "att1: hello", values)
}

func TestGetHelmIndex(t *testing.T) {
	subscription := &appv1alpha1.HelmChartSubscription{}
	err := yaml.Unmarshal([]byte(helmRepoSub), subscription)
	assert.NoError(t, err)

	subscriber := &HelmRepoSubscriber{
		HelmChartSubscription: subscription,
	}

	indexFile, hash, err := subscriber.getHelmRepoIndex()
	assert.NoError(t, err)

	assert.NotEqual(t, "", hash)

	assert.Equal(t, 2, len(indexFile.Entries))
}

func TestGenerateHelmIndexYAML(t *testing.T) {
	subscription := &appv1alpha1.HelmChartSubscription{}
	err := yaml.Unmarshal([]byte(gitRepoSub), subscription)
	assert.NoError(t, err)

	subscriber := &HelmRepoSubscriber{
		HelmChartSubscription: subscription,
	}

	indexFile, hash, err := subscriber.generateIndexYAML()
	assert.NoError(t, err)

	assert.NotEqual(t, "", hash)

	assert.Equal(t, 2, len(indexFile.Entries))
}
