# Deployment Guide

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Deployment Guide](#deployment-guide)
    - [Environment variable](#environment-variable)
    - [RBAC](#rbac)
        - [Deployment](#deployment)
    - [General process](#general-process)
<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Environment variable

The environment variable `CHARTS_DIR` must be set when developing, it specifies the directory where the charts will be downloaded and expanded (Default `/tmp/charts`).

## RBAC

The service account is `multicloud-operators-subscription-release`.

The cluster-role `cluster-admin` is binded to the service account as the helmrelease operator must be able to deploy helm chart in any namespace.

The role `multicloud-operators-subscription-release` is binded to that service account.

A cluster role `multicloud-operators-subscription-release` is created for the `helmrelease` resource.

In order for another service account to be able to access the `helmrelease`, a role binding must be create.

### Deployment

1) Do `kubectl apply -f` on all files in deploy/crds.*-crd.yaml
2) `kubectl apply -f service_account.yaml`
3) `kubectl apply -f role.yaml`
4) `kubectl apply -f role_binding.yaml`
5) `kubectl apply -f operator.yaml`

## General process

Helmrelease CR:

```yaml
apiVersion: multicloud-apps.io/v1
kind: HelmRelease
metadata:
  name: myapp-ibm-myapp-api-ope
  namespace: default
spec:
  source:
    type: helmrepo
    helmRepo:
      URLs:
      - https://mycluster.icp:8443/helm-repo/requiredAssets/ibm-myapp-api-0.2.3-015-20190725140717.tgz
  chartName: ibm-myapp-api
  secretRef:
    name: mysecret
  configRef:
    name: mycluster-config
    value: |
      attribute1: value1
      attribute2: value2
```

`file:` sheme is also supported to define the location of a local file.

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
