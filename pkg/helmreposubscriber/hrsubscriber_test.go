// IBM Confidential
// OCO Source Materials
// 5737-E67
// (C) Copyright IBM Corporation 2019 All Rights Reserved
// The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.

package helmreposubscriber

import (
	"testing"

	operatorv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
)

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
    - ICP
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
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{
				SubscriptionSpec: operatorv1alpha1.SubscriptionSpec{
					Package: "ibm-cfee-installer",
				},
			},
		},
	}
	s.filterCharts(indexFile)
	assert.Equal(t, 1, len(indexFile.Entries))
	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingWithoutPackageName(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)
	s := &HelmRepoSubscriber{
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{
				SubscriptionSpec: operatorv1alpha1.SubscriptionSpec{},
			},
		},
	}
	s.filterCharts(indexFile)
	assert.Equal(t, 3, len(indexFile.Entries))
	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingWithoutOPSubscriptionSpec(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)
	s := &HelmRepoSubscriber{
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{},
		},
	}
	s.filterCharts(indexFile)
	assert.Equal(t, 3, len(indexFile.Entries))
	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingWithoutSubscriptionSpec(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)
	s := &HelmRepoSubscriber{
		Subscription: &appv1alpha1.Subscription{},
	}
	s.filterCharts(indexFile)
	assert.Equal(t, 3, len(indexFile.Entries))
	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingDigest(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)
	s := &HelmRepoSubscriber{
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					Annotations: map[string]string{
						"digest": "c3a86622131b877863bf292a2c991a66a5c1132a2db3297eea8c423cd001ab24",
					},
				},
			},
		},
	}
	s.filterCharts(indexFile)
	assert.Equal(t, 1, len(indexFile.Entries))
	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(chartVersions))
}

func Test_MatchingTillerVersion(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)
	s := &HelmRepoSubscriber{
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					Annotations: map[string]string{
						"tillerVersion": "2.4.0",
					},
				},
			},
		},
	}
	s.filterCharts(indexFile)
	assert.Equal(t, 1, len(indexFile.Entries))
	versionedCharts := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 2, len(versionedCharts))
}

func Test_MatchingTillerVersionNotFound(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)
	s := &HelmRepoSubscriber{
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					Annotations: map[string]string{
						"tillerVersion": "2.2.0",
					},
				},
			},
		},
	}
	s.filterCharts(indexFile)
	assert.Equal(t, 0, len(indexFile.Entries))
}

func Test_MatchingVersion(t *testing.T) {
	indexFile, err := LoadIndex([]byte(index))
	assert.NoError(t, err)
	s := &HelmRepoSubscriber{
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{
				PackageFilter: &appv1alpha1.PackageFilter{
					Version: ">=3.1.3 <=3.2.0-alpha",
				},
			},
		},
	}
	s.filterCharts(indexFile)
	assert.Equal(t, 2, len(indexFile.Entries))
	versionedCharts := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 1, len(versionedCharts))
	assert.Equal(t, "3.2.0-alpha", versionedCharts[0].GetVersion())
	versionedChartsNil := indexFile.Entries["ibm-mcm-prod"]
	assert.Nil(t, versionedChartsNil)
	versionedCharts = indexFile.Entries["ibm-mcmk-prod"]
	assert.Equal(t, 1, len(versionedCharts))
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
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{
				SubscriptionSpec: operatorv1alpha1.SubscriptionSpec{
					Package: "ibm-mcm-prod",
				},
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
		Subscription: &appv1alpha1.Subscription{
			Spec: appv1alpha1.SubscriptionSpec{
				SubscriptionSpec: operatorv1alpha1.SubscriptionSpec{
					Package: "ibm-cfee-installer",
				},
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

const subWithOverrides = `apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  name: dev-sub-with-overrides
  namespace: default
spec:
  channel: default/test
  name: ibm-cfee-installer
  packageOverrides:
  - packageName: ibm-cfee-installer
    packageOverrides:
    - path: spec.values
      value: |
        TestValue: 
          att1: val1
          att2: val2
`

const subWithOutOverrides = `apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  name: dev-sub-with-overrides
  namespace: default
spec:
  channel: default/test
  name: ibm-cfee-installer
`
