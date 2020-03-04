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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	appv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
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

var (
	configMapName = "cm-helmoutils"
	configMapNS   = "default"
	secretName    = "secret-helmoutils"
	secretNS      = "default"
)

func TestGetConfig(t *testing.T) {
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

	c := mgr.GetClient()

	configMapRef := &corev1.ObjectReference{
		Name:      configMapName,
		Namespace: configMapNS,
	}

	configMapResp, err := GetConfigMap(c, configMapNS, configMapRef)
	assert.NoError(t, err)

	assert.Nil(t, configMapResp)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNS,
		},
		Data: map[string]string{
			"att1": "att1value",
			"att2": "att2value",
		},
	}

	err = c.Create(context.TODO(), configMap)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	configMapResp, err = GetConfigMap(c, configMapNS, configMapRef)
	assert.NoError(t, err)

	assert.NotNil(t, configMapResp)
	assert.Equal(t, "att1value", configMapResp.Data["att1"])
}

func TestSecret(t *testing.T) {
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

	c := mgr.GetClient()

	secretRef := &corev1.ObjectReference{
		Name:      secretName,
		Namespace: secretNS,
	}

	secretResp, err := GetSecret(c, secretNS, secretRef)
	assert.Error(t, err)

	assert.Nil(t, secretResp)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNS,
		},
		Data: map[string][]byte{
			"att1": []byte("att1value"),
			"att2": []byte("att2value"),
		},
	}

	err = c.Create(context.TODO(), secret)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	secretResp, err = GetSecret(c, secretNS, secretRef)
	assert.NoError(t, err)

	assert.NotNil(t, secretResp)
	assert.Equal(t, []byte("att1value"), secretResp.Data["att1"])
}

func TestDownloadChartGitHub(t *testing.T) {
	hr := &appv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.GitHubSourceType,
				GitHub: &appv1.GitHub{
					Urls:      []string{"https://github.com/open-cluster-management/multicloud-operators-subscription-release.git"},
					ChartPath: "test/github/subscription-release-test-1",
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	destDir, err := DownloadChart(nil, nil, dir, hr)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(destDir, "Chart.yaml"))
	assert.NoError(t, err)
}

func TestDownloadChartHelmRepo(t *testing.T) {
	hr := &appv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.HelmRepoSourceType,
				HelmRepo: &appv1.HelmRepo{
					Urls: []string{
						"https://raw.github.com/open-cluster-management/multicloud-operators-subscription-release/master/test/helmrepo/subscription-release-test-1-0.1.0.tgz"},
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	destDir, err := DownloadChart(nil, nil, dir, hr)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(destDir, "Chart.yaml"))
	assert.NoError(t, err)
}

func TestDownloadChartHelmRepoContainsInvalidURL(t *testing.T) {
	hr := &appv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.HelmRepoSourceType,
				HelmRepo: &appv1.HelmRepo{
					Urls: []string{
						"https://raw.github.com/open-cluster-management/multicloud-operators-subscription-release/master/test/helmrepo/subscription-release-test-1-0.1.0.tgz",
						"https://badURL1"},
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	destDir, err := DownloadChart(nil, nil, dir, hr)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(destDir, "Chart.yaml"))
	assert.NoError(t, err)
}

func TestDownloadChartHelmRepoContainsInvalidURL2(t *testing.T) {
	hr := &appv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.HelmRepoSourceType,
				HelmRepo: &appv1.HelmRepo{
					Urls: []string{"https://badURL1",
						"https://raw.github.com/open-cluster-management/multicloud-operators-subscription-release/master/test/helmrepo/subscription-release-test-1-0.1.0.tgz"},
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	destDir, err := DownloadChart(nil, nil, dir, hr)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(destDir, "Chart.yaml"))
	assert.NoError(t, err)
}

func TestDownloadChartHelmRepoAllInvalidURLs(t *testing.T) {
	hr := &appv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.HelmRepoSourceType,
				HelmRepo: &appv1.HelmRepo{
					Urls: []string{"https://badURL1", "https://badURL2", "https://badURL3", "https://badURL4", "https://badURL5"},
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	_, err = DownloadChart(nil, nil, dir, hr)
	assert.Error(t, err)
}

func TestDownloadChartFromGitHub(t *testing.T) {
	hr := &appv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.GitHubSourceType,
				GitHub: &appv1.GitHub{
					Urls:      []string{"https://github.com/open-cluster-management/multicloud-operators-subscription-release.git"},
					ChartPath: "test/github/subscription-release-test-1",
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	destDir, err := DownloadChartFromGitHub(nil, nil, dir, hr)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(destDir, "Chart.yaml"))
	assert.NoError(t, err)
}

func TestDownloadChartFromHelmRepoHTTP(t *testing.T) {
	hr := &appv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.HelmRepoSourceType,
				HelmRepo: &appv1.HelmRepo{
					Urls: []string{
						"https://raw.github.com/open-cluster-management/multicloud-operators-subscription-release/master/test/helmrepo/subscription-release-test-1-0.1.0.tgz"},
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	chartDir, err := DownloadChartFromHelmRepo(nil, nil, dir, hr)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(chartDir, "Chart.yaml"))
	assert.NoError(t, err)
}

func TestDownloadChartFromHelmRepoLocal(t *testing.T) {
	hr := &appv1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "subscription-release-test-1-cr",
			Namespace: "default",
		},
		Repo: appv1.HelmReleaseRepo{
			Source: &appv1.Source{
				SourceType: appv1.HelmRepoSourceType,
				HelmRepo: &appv1.HelmRepo{
					Urls: []string{"file:../../test/helmrepo/subscription-release-test-1-0.1.0.tgz"},
				},
			},
			ChartName: "subscription-release-test-1",
		},
	}
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	chartDir, err := DownloadChartFromHelmRepo(nil, nil, dir, hr)
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(chartDir, "Chart.yaml"))
	assert.NoError(t, err)
}

func TestDownloadGitHubRepo(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	destRepo := filepath.Join(dir, "test")
	commitID, err := DownloadGitHubRepo(nil, nil, destRepo,
		[]string{"https://github.com/open-cluster-management/multicloud-operators-subscription-release.git"}, "")
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(destRepo, "OWNERS"))
	assert.NoError(t, err)

	assert.NotEqual(t, commitID, "")
}

func TestKeywordsChecker(t *testing.T) {
	req0 := metav1.LabelSelectorRequirement{
		Key:      "l1",
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{"true"},
	}
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"l1": "true",
			"l2": "true",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			req0,
		},
	}
	ks := []string{"l1", "l2"}
	b := KeywordsChecker(labelSelector, ks)
	assert.Equal(t, true, b)

	//No keywords
	ks = nil
	b = KeywordsChecker(labelSelector, ks)
	assert.Equal(t, false, b)

	//No keywords and no selector
	labelSelector = nil
	b = KeywordsChecker(labelSelector, ks)
	assert.Equal(t, true, b)

	//Keywords and no selector
	ks = []string{"l1", "l2"}
	b = KeywordsChecker(labelSelector, ks)
	assert.Equal(t, true, b)
}

func TestUnmarshalIndex(t *testing.T) {
	indexFile, err := UnmarshalIndex([]byte(index))
	assert.NoError(t, err)

	chartVersions := indexFile.Entries["ibm-cfee-installer"]
	assert.Equal(t, 2, len(chartVersions))

	name := chartVersions[1].GetName()
	assert.Equal(t, "ibm-cfee-installer", name)
}
func TestGetHelmIndex(t *testing.T) {
	indexFile, hash, err := GetHelmRepoIndex(nil, nil, "",
		[]string{"https://raw.github.com/open-cluster-management/multicloud-operators-subscription-release/master/test/helmrepo"})
	assert.NoError(t, err)

	assert.NotEqual(t, "", hash)

	assert.Equal(t, 2, len(indexFile.Entries))
}

func TestGenerateHelmIndexYAML(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "charts")
	assert.NoError(t, err)

	defer os.RemoveAll(dir)

	destRepo := filepath.Join(dir, "test")
	indexFile, hash, err := GenerateGitHubIndexFile(nil, nil,
		destRepo,
		[]string{"https://github.com/open-cluster-management/multicloud-operators-subscription-release.git"},
		"test/github", "")
	assert.NoError(t, err)

	assert.NotEqual(t, "", hash)

	assert.Equal(t, 2, len(indexFile.Entries))
}
