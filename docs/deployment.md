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

The service account is `multicluster-operators-subscription-release`.

The cluster-role `cluster-admin` is binded to the service account as the helmrelease operator must be able to deploy helm chart in any namespace.

The role `multicluster-operators-subscription-release` is binded to that service account.

A cluster role `multicluster-operators-subscription-release` is created for the `helmrelease` resource.

In order for another service account to be able to access the `helmrelease`, a role binding must be create.

### Deployment

```shell
cd multicloud-operators-subscription-release
kubectl apply -f deploy/crds
kubectl apply -f deploy
```

## General process

Helmrelease CR:

```yaml
apiVersion: apps.open-cluster-management.io/v1
kind: HelmRelease
metadata:
  name: nginx-ingress
  namespace: default
repo:
  chartName: nginx-ingress
  source:
    helmRepo:
      urls:
      - https://kubernetes-charts.storage.googleapis.com/nginx-ingress-1.26.0.tgz
    type: helmrepo
  version: 1.26.0
spec:
  defaultBackend:
    replicaCount: 1
```

`file:` sheme is also supported to define the location of a local file.

Source can have the following format for github:

```yaml
  source:
    github:
      urls:
      - https://github.com/helm/charts
      chartPath: stable/nginx-ingress
      branch: master
    type: github
```
