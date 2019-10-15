# multicloud-operators-subscription-release

The multicloud-operators-subscription-release is composed of 2 controllers. The helmchartsubscription controller which is in charge of managing the helmchartsubscription CR. That CR defines the location of the charts (helmrepo or github) and filters to select a subset of charts to deploy. the helmchartsubscription controller will then create a number of helmreleases and these are managed by the helmrelease controller. The helmrelease controller will manage the helmrelease CR, download the chart from the helmrepo or github and then call the operator-sdk helm-operator methods to start the deployment of each chart.

The helmrelease controller can be use independently without the helmchartsubscription controller. The flag `--helmchart-subscription-controller-disabled` can be used to disable the helmchartsubscription controller.

## Environment variable

The environment variable `CHARTS_DIR` must be set when developing, it specifies the directory where the charts will be downloaded and expanded.

## Launch Dev mode
```
operator-sdk up local --verbose [--operator-flags "--helmchart-subscription-controller-disabled"]
```

## Build

operator-sdk build ibm/multicloud-operators-subscription-release:latest
docker tag ibm/multicloud-operators-subscription-release:latest mycluster.icp:8500/kube-system/ibm/multicloud-operators-subscription-release:latest
docker push mycluster.icp:8500/kube-system/ibm/multicloud-operators-subscription-release:latest

## Environment deployment

TODO to improve for ClusterRole

1) Do `kubectl apply -f` on all files in deploy/crds.*-crd.yaml
2) `kubectl apply -f service_account.yaml`
3) `kubectl apply -f role.yaml`
4) `kubectl apply -f role_binding.yaml`
5) `kubectl apply -f operator.yaml`

## General process
The operator generates `HelmRelease` CR for each chart to deploy in the same namespace and named `<s.Name>-<chart_name>[-<channel_name>]`. The channel_name is added only if the channel attribute is set in the subscription.

To do so, the following steps are taken:

1) Read the index.yaml at the source address.
2) Filter the index.yaml with the spec.Name and spec.packageFilter.
3) Take the last version of a chart if multiple version are still present for the same chart after filtering.
4) Create a HelmRelease for each entries in the filtered index.yaml

## HelmChartSubscriptions

The subscription operator watches `HelmChartSubscription` and `HelmRelease` CRs.

The User creates a HelmChartSubscription CR. if installPlanApproval is set to `Automatic` then the helmrepo will be monitored and new chart version will be deployed, if set to `Manual` then no automatic deployment.


```yaml
apiVersion: app.ibm.com/v1alpha1
kind: HelmChartSubscription
metadata:
  name: razee
  namespace: default
spec:
  channel: default/ope
  installPlanApproval: Automatic
  secretRef:
    name: mysecret
  configRef:
    name: mycluster-config
  name: ibm-razee-api
  packageFilter:
    keywords:
    - ICP
    annotations:
      tillerVersion: 2.4.0
    version: '>0.2.2'
  packageOverrides:
  - packageName: ibm-razee-api
    packageOverrides:
    - path: spec.values
      value: "RazeeAPI: \n  Endpoint: http://9.30.166.165:31311\n  ObjectstoreSecretName:
        minio\n  Region: us-east-1\n"
  chartsSource:
    helmrepo:
      urls:
      - https://mycluster.icp:8443/helm-repo/charts
  ```
  
  Source can have the following format for github (not yet fully implemented):

  ``` yaml
  chartsSource:
    type: github
    github:
      urls:
      - https://github.ibm.com/IBMPrivateCloud/hybrid-cluster-manager-v2-chart.git
      chartsPath: 3.2.1-examples/guestbook-kube-subscription
      branch: master
  ```

branch master is the default.

## Helm-charts filtering

The optional spec.name defines the name of the helm-chart, it can be also a regex if multiple helm-charts must be deployed.

The optional spec.packageFilter allows to filter the helm-charts.
Filtering is done on:

- the version of the helm-chart (semver expression), 
- the tiller version of the helm-chart (Should may be removed as the operator has its own tiller)
- the digest must match
- the keywords, if the helm-chart has a least 1 listed keywords then it eligible for deployment.

## Authentication

A secretRef can be provided in the subscriptionRelease spec. It references a secret where the authentication parameter to access the helm-repo are set.
The attributes are either `user` and `password` or `authHeader`. All values must be base64 encoded.
The `authHeader` format is `<Auth_type> <token>` and so for example: 
`Bearer xxxxxx`.

## Helm-repo client configuration

The configRef is a reference to a configMap which holds the parameters to the helm-repo.

```yaml
apiVersion: v1
data:
  insecureSkipVerify: "true"
kind: ConfigMap
metadata:
  name: mycluster-config
  namespace: default
```


The HelmReleases are owned by the HelmChartSubscription and so if the subscription is deleted the release is deleted too.

```yaml
apiVersion: app.ibm.com/v1alpha1
kind: HelmRelease
metadata:
  annotations:
    app.ibm.com/hosting-deployable: default/ope
    app.ibm.com/hosting-subscription: default/razee
  creationTimestamp: 2019-08-12T09:01:52Z
  generation: 1
  name: razee-ibm-razee-api-ope
  namespace: default
  ownerReferences:
  - apiVersion: app.ibm.com/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: HelmChartSubscription
    name: razee
    uid: ec3c8f28-bcde-11e9-b55f-fa163e0cb658
  resourceVersion: "3852059"
  selfLink: /apis/app.ibm.com/v1alpha1/namespaces/default/subscriptionreleases/razee-ibm-razee-api-ope
  uid: d35adca8-bcdf-11e9-b55f-fa163e0cb658
spec:
  source:
    type: helmrepo
    helmRepo:
      URLs:
      - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-razee-api-0.2.3-015-20190725140717.tgz
  chartName: ibm-razee-api
  secretRef:
    name: mysecret
  configRef:
    name: mycluster-config
  values: "RazeeAPI: \n  Endpoint: http://9.30.166.165:31311\n  ObjectstoreSecretName:
    minio\n  Region: us-east-1\n"
  version: 0.2.3-015-20190725140717
```

Source can have the following format for github:

```yaml
  source:
    github:
      urls:
      - https://github.ibm.com/IBMPrivateCloud/icp-cert-manager-chart
      chartPath: stable/ibm-cert-manager
      branch: master
    type: github
```

Branch master is the default.

Once the HelmRelease is created or modified, the operator will deploy each charts specified in each HelmRelease.

To do so, the following steps are taken:

1) Download the chart tgz in the `$CHARTS_DIR`.
2) Unzip the tgz in `$CHARTS_DIR/<sr.Spec.ReleaseName>/<sr.namespace>/<chart_name>`
3) Create a manager with the values provided in the HelmRelease
4) Launch the deployment.
