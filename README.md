# subscription-operator

The subscription operator watches `Subscription` and `SubscriptionRelease` CRs.

The User creates a Subscription CR.

```yaml
apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  name: dev-sub-razee-ope
  namespace: default
spec:
  channel: default/ope
  name: ibm-razee-api
  packageFilter:
    annotations:
      tillerVersion: 2.4.0
    version: '>0.2.2'
  packageOverrides:
  - packageName: ibm-razee-api
    packageOverrides:
    - path: spec.values
      value: "RazeeAPI: \n  Endpoint: http://9.30.166.165:31311\n  ObjectstoreSecretName:
        minio\n  Region: us-east-1\n"
  source: https://mycluster.icp:8443/helm-repo/charts
```

The operator generates `SubscriptionRelease` CR for each chart to deploy in the same namespace and named `<subscription_name>-<chart_name>`.

To do so, the following steps are taken:

1) Read the index.yaml at the source address.
2) Filter the index.yaml with the spec.Name and spec.packageFilter.
3) Take the last version of a chart if multiple version are still present for the same chart after filtering.
4) Create a SubscriptionRelease for each entries in the filtered index.yaml

The SubscriptionReleases are owned by the Subscription and so if the subscription is deleted the release is deleted too.


```yaml
apiVersion: app.ibm.com/v1alpha1
kind: SubscriptionRelease
metadata:
  creationTimestamp: 2019-08-07T09:15:15Z
  generation: 1
  labels:
    app: dev-sub-razee-ope
    subscriptionName: dev-sub-razee-ope
    subscriptionNamespace: default
  name: dev-sub-razee-ope-ibm-razee-api
  namespace: default
  ownerReferences:
  - apiVersion: app.ibm.com/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Subscription
    name: dev-sub-razee-ope
    uid: b5ad9b54-b870-11e9-b55f-fa163e0cb658
  resourceVersion: "2986792"
  selfLink: /apis/app.ibm.com/v1alpha1/namespaces/default/subscriptionreleases/dev-sub-razee-ope-ibm-razee-api
  uid: de3a5eb4-b8f3-11e9-b55f-fa163e0cb658
spec:
  URLs:
  - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-razee-api-0.2.3-015-20190725140717.tgz
  chartName: ibm-razee-api
  releaseName: ibm-razee-api
  values: "RazeeAPI: \n  Endpoint: http://9.30.166.165:31311\n  ObjectstoreSecretName:
    minio\n  Region: us-east-1\n"
  version: 0.2.3-015-20190725140717
```

Once the SubscriptionRelease is created or modified, the operator will deploy each charts specified in each SubscriptionRelease.

To do so, the following steps are taken:

1) Download the chart tgz in the `$CHARTS_DIR`.
2) Unzip the tgz in `$CHARTS_DIR/<subscription<subscription_namespace>/<chart_name>`
3) Merge the values provided in the SubscriptionRelease with the values.yaml present in the chart.
4) Launch the deployment.

## Environment variable

The environment variable `CHARTS_DIR` must be set, it specifies the director where the charts will be downloaded and expanded.

## Launch Dev mode

operator-sdk up local --verbose

