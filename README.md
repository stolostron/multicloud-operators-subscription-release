# subscription-operator

## Environment variable

The environment variable `CHARTS_DIR` must be set when developping, it specifies the directory where the charts will be downloaded and expanded.

## Launch Dev mode

operator-sdk up local --verbose

## Build

operator-sdk build ibm/subscription-operator:latest
docker tag ibm/subscription-operator:latest mycluster.icp:8500/kube-system/ibm/subscription-operator:latest
docker push mycluster.icp:8500/kube-system/ibm/subscription-operator:latest

## Environment deployment

TODO to improve for ClusterRole

1) Do `kubectl apply -f` on all files in deploy/crds.*-crd.yaml
2) `kubectl apply -f service_account.yaml`
3) `kubectl apply -f role.yaml`
4) `kubectl apply -f role_binding.yaml`
5) `kubectl apply -f operator.yaml`

## General process
The operator generates `SubscriptionRelease` CR for each chart to deploy in the same namespace and named `<s.Name>-<chart_name>[-<channel_name>]`. The channel_name is added only if the channel attribute is set in the subscription.

To do so, the following steps are taken:

1) Read the index.yaml at the source address.
2) Filter the index.yaml with the spec.Name and spec.packageFilter.
3) Take the last version of a chart if multiple version are still present for the same chart after filtering.
4) Create a SubscriptionRelease for each entries in the filtered index.yaml

## Subscriptions

The subscription operator watches `Subscription` and `SubscriptionRelease` CRs.

The User creates a Subscription CR.

```yaml
apiVersion: app.ibm.com/v1alpha1
kind: Subscription
metadata:
  name: razee
  namespace: default
spec:
  channel: default/ope
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
  source: https://mycluster.icp:8443/helm-repo/charts
  ```
  
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


The SubscriptionReleases are owned by the Subscription and so if the subscription is deleted the release is deleted too.

```yaml
apiVersion: app.ibm.com/v1alpha1
kind: SubscriptionRelease
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
    kind: Subscription
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
  releaseName: razee-ibm-razee-api-ope
  values: "RazeeAPI: \n  Endpoint: http://9.30.166.165:31311\n  ObjectstoreSecretName:
    minio\n  Region: us-east-1\n"
  version: 0.2.3-015-20190725140717
```

Once the SubscriptionRelease is created or modified, the operator will deploy each charts specified in each SubscriptionRelease.

To do so, the following steps are taken:

1) Download the chart tgz in the `$CHARTS_DIR`.
2) Unzip the tgz in `$CHARTS_DIR/<sr.Spec.ReleaseName>/<sr.namespace>/<chart_name>`
3) Create a manager with the values provided in the SubscriptionRelease
4) Launch the deployment.

The releaseName is the concatanation of the `<subscription-name>-<chartName>-<channel-name>`. The current implementation of the helm-operator which containates to the releasename a UUID and so some generated resources will have a longueur name than 52 characters such as `<releasename>-<UUID>-delete-registrations`. To avoid that issue the releasename will be shorten to 52-25-1-21=5.

52: max available chars.
25: UUID length
1: dash separator
21: length of `-delete-registrations`
