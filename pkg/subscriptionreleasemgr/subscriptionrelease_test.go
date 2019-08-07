package subscriptionreleasemgr

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	appv1alpha1 "github.ibm.com/IBMMulticloudPlatform/subscription-operator/pkg/apis/app/v1alpha1"
)

const index = `
apiVersion: v1
entries:
  ibm-razee-api:
  - created: 2019-07-25T14:59:42.541350233Z
    description: Razee API
    digest: e5a8e6c80c4885af0804f4097a09db7a73d5c153415b5d8d58716e4c661a7799
    icon: https://www.ibm.com/cloud-computing/images/new-cloud/img/cloud.png
    keywords:
    - amd64
    - DevOps
    - Development
    - ICP
    - Tech
    name: ibm-razee-api
    tillerVersion: '>=2.4.0'
    urls:
    - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-razee-api-0.2.3-015-20190725140717.tgz
    version: 0.2.3-015-20190725140717
  - created: 2019-07-25T14:59:42.447720534Z
    description: Razee API
    digest: ffa1c247827aa06c58999de6df6d36746dfaa9eefae033f565e9aa488829560c
    icon: https://www.ibm.com/cloud-computing/images/new-cloud/img/cloud.png
    keywords:
    - amd64
    - DevOps
    - Development
    - ICP
    - Tech
    name: ibm-razee-api
    tillerVersion: '>=2.4.0'
    urls:
    - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-razee-api-0.2.3-014-20190725131437.tgz
    version: 0.2.3-014-20190725131437
  - created: 2019-07-25T14:59:42.446045539Z
    description: Razee API
    digest: 1510427022f6b2a45e25a1dd5106bf425f038fb828474f64cece545abe23f10a
    icon: https://www.ibm.com/cloud-computing/images/new-cloud/img/cloud.png
    keywords:
    - amd64
    - DevOps
    - Development
    - ICP
    - Tech
    name: ibm-razee-api
    tillerVersion: '>=2.4.0'
    urls:
    - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-razee-api-0.2.3-013-20190725012204.tgz
    version: 0.2.3-013-20190725012204
  - created: 2019-07-25T14:59:42.444642983Z
    description: Razee API
    digest: 8cc226c9f1d1ec472c3f1b58142cb8c3d98b33e4cc5f8fa55b46d3a69a9953cd
    icon: https://www.ibm.com/cloud-computing/images/new-cloud/img/cloud.png
    keywords:
    - amd64
    - DevOps
    - Development
    - ICP
    - Tech
    name: ibm-razee-api
    tillerVersion: '>=2.4.0'
    urls:
    - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-razee-api-0.2.2-013-20190717154729.tgz
    version: 0.2.2-013-20190717154729
generated: 2019-07-25T14:59:42.443201016Z
`

const sub = `apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  annotations:
    tillerVersion: 2.4.0
  name: dev-sub-razee
  namespace: default
  resourceVersion: "1798769"
  selfLink: /apis/app.ibm.com/v1alpha1/namespaces/default/subscriptions/dev-sub-razee
  uid: 1475377b-aeed-11e9-b55f-fa163e0cb658
spec:
  channel: default/test
  name: ibm-razee-api
  packageFilter:
    annotations:
      tillerVersion: 2.4.0
    version: ">0.2.2"
  packageOverrides:
  - packageName: ibm-razee-api
    packageOverrides:
    - path: spec.values
      value: "RazeeAPI: \n  Endpoint: http://9.30.166.165:31311\n  ObjectstoreSecretName:
        minio\n  Region: us-east-1\n"
`

const sr = `
apiVersion: app.ibm.com/v1alpha1
kind: SubscriptionRelease
metadata:
  creationTimestamp: 2019-08-06T11:33:55Z
  generation: 1
  labels:
    app: dev-sub-razee-ope
    subscriptionName: dev-sub-razee-ope
    subscriptionNamespace: default
  name: dev-sub-razee-ope-sr
  namespace: default
  ownerReferences:
  - apiVersion: app.ibm.com/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Subscription
    name: dev-sub-razee-ope
    uid: 293b92e4-b763-11e9-b55f-fa163e0cb658
  resourceVersion: "2746558"
  selfLink: /apis/app.ibm.com/v1alpha1/namespaces/default/subscriptionreleases/dev-sub-razee-ope-sr
  uid: 130a26f2-b83e-11e9-b55f-fa163e0cb658
spec:
  chartName: ibm-razee-api
  releaseName: ibm-razee-api
  repoUrl: https://mycluster.icp:8443/helm-repo/charts
  version: 0.2.3-015-20190725140717
`

// const index = `apiVersion: v1
// entries:
//   nginx-ingress:
//   - appVersion: 1.5.2
//     created: "2019-07-31T14:29:16.561859185Z"
//     description: NGINX Ingress Controller
//     digest: 02089cbfc65e684c4943f29c971e4affbffe05fea88328e5c10011c6e2a46da4
//     icon: https://raw.githubusercontent.com/nginxinc/kubernetes-ingress/v1.5.2/deployments/helm-chart/chart-icon.png
//     keywords:
//     - ingress
//     - nginx
//     maintainers:
//     - email: kubernetes@nginx.com
//       name: NGINX Kubernetes Team
//     name: nginx-ingress
//     sources:
//     - https://github.com/nginxinc/kubernetes-ingress/tree/v1.5.2/deployments/helm-chart
//     urls:
//     - https://helm.nginx.com/stable/nginx-ingress-0.3.2.tgz
//     version: 0.3.2
//   - appVersion: 1.5.1
//     created: "2019-07-31T14:29:16.561335791Z"
//     description: NGINX Ingress Controller
//     digest: 9c59c9ca99c0894a9db24ee6f842bd99304a29ca64b5c57356c1332c701a8e64
//     icon: https://raw.githubusercontent.com/nginxinc/kubernetes-ingress/v1.5.1/deployments/helm-chart/chart-icon.png
//     keywords:
//     - ingress
//     - nginx
//     maintainers:
//     - email: kubernetes@nginx.com
//       name: NGINX Kubernetes Team
//     name: nginx-ingress
//     sources:
//     - https://github.com/nginxinc/kubernetes-ingress/tree/v1.5.1/deployments/helm-chart
//     urls:
//     - https://helm.nginx.com/stable/nginx-ingress-0.3.1.tgz
//     version: 0.3.1
//   - appVersion: 1.5.0
//     created: "2019-07-31T14:29:16.560786527Z"
//     description: NGINX Ingress Controller
//     digest: c205aaa25a641353f3c255c99b18bafe150267b8dc4a9ac276c1e3dab1cc83ee
//     icon: https://raw.githubusercontent.com/nginxinc/kubernetes-ingress/v1.5.0/deployments/helm-chart/chart-icon.png
//     keywords:
//     - ingress
//     - nginx
//     maintainers:
//     - email: kubernetes@nginx.com
//       name: NGINX Kubernetes Team
//     name: nginx-ingress
//     sources:
//     - https://github.com/nginxinc/kubernetes-ingress/tree/v1.5.0/deployments/helm-chart
//     urls:
//     - https://helm.nginx.com/stable/nginx-ingress-0.3.0.tgz
//     version: 0.3.0
//   - appVersion: 1.4.6
//     created: "2019-07-31T14:29:16.560279903Z"
//     description: NGINX Ingress Controller
//     digest: 1c40fb925dcc19fb24b6af864400642360e188f2ee2b63c029b5441c0a906160
//     icon: https://raw.githubusercontent.com/nginxinc/kubernetes-ingress/v1.4.6/deployments/helm-chart/chart-icon.png
//     keywords:
//     - ingress
//     - nginx
//     maintainers:
//     - email: kubernetes@nginx.com
//       name: NGINX Kubernetes Team
//     name: nginx-ingress
//     sources:
//     - https://github.com/nginxinc/kubernetes-ingress/tree/v1.4.6/deployments/helm-chart
//     urls:
//     - https://helm.nginx.com/stable/nginx-ingress-0.2.1.tgz
//     version: 0.2.1
//   - appVersion: 1.4.5
//     created: "2019-07-31T14:29:16.559837451Z"
//     description: NGINX Ingress Controller
//     digest: 5c7f7badb8cf5bc7f36f0b770dfd4d232109623e6fbe7fd5907fb82243245c0d
//     icon: https://raw.githubusercontent.com/nginxinc/kubernetes-ingress/v1.4.5/deployments/helm-chart/chart-icon.png
//     keywords:
//     - ingress
//     - nginx
//     maintainers:
//     - email: kubernetes@nginx.com
//       name: NGINX Kubernetes Team
//     name: nginx-ingress
//     sources:
//     - https://github.com/nginxinc/kubernetes-ingress/tree/v1.4.5/deployments/helm-chart
//     urls:
//     - https://helm.nginx.com/stable/nginx-ingress-0.2.0.tgz
//     version: 0.2.0
// generated: "2019-07-31T14:29:16.559225141Z"
// `
// const sub = `apiVersion: app.ibm.com/v1alpha1
// kind: Subscription
// metadata:
//   name: ngnix
//   namespace: default
// spec:
//   channel: default/ngnix
//   name: nginx-ingress
//   packageFilter:
//     annotations:
//       tillerVersion: 2.4.0
//     version: ">=0.3.1"
//   packageOverrides:
//   - packageName: nginx-ingress
//     packageOverrides:
//     - path: spec.values
//       value: |
//         replicaCount: 2
// `

func TestRelease(t *testing.T) {
	var s appv1alpha1.SubscriptionRelease

	err := yaml.Unmarshal([]byte(sr), &s)
	assert.NoError(t, err)
	mgr, err := NewHelmManager(s)
	assert.NoError(t, err)
	err = mgr.Sync(context.TODO())
	assert.NoError(t, err)
	_, err = mgr.InstallRelease(context.TODO())
	assert.NoError(t, err)

}

func Test_RetrieveOverride(t *testing.T) {
	var s appv1alpha1.Subscription
	err := yaml.Unmarshal([]byte(sub), &s)
	assert.NoError(t, err)
	for _, packageElem := range s.Spec.PackageOverrides {
		for _, pathElem := range packageElem.PackageOverrides {
			data, err := pathElem.MarshalJSON()
			assert.NoError(t, err)
			fmt.Print(string(data))
			var m map[string]interface{}
			err = json.Unmarshal(data, &m)
			assert.NoError(t, err)
			if m["path"] == "spec.values" {
				fmt.Print(m["value"])
			}
		}
	}
	t.FailNow()
}
