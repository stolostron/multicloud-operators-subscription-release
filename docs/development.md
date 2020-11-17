# Development Guide

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Development Guide](#development-guide)
    - [Environment variable](#environment-variable)
    - [Launch Dev mode](#launch-dev-mode)
    - [Build a local image](#build-a-local-image)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Environment variable

The environment variable `CHARTS_DIR` must be set when developing, it specifies the directory where the charts will be downloaded and expanded (Default `/tmp/charts`).

## Launch Dev mode

```shell
git clone git@github.com:open-cluster-management/multicloud-operators-subscription-release.git
cd multicloud-operators-subscription-release
export GITHUB_USER=<github_user>
export GITHUB_TOKEN=<github_token>
make
make build
kubectl apply -f deploy/crds
./build/_output/bin/multicluster-operators-subscription-release
```

## Build a local image

```shell
git clone git@github.com:open-cluster-management/multicloud-operators-subscription-release.git
cd multicloud-operators-subscription-release
export GITHUB_USER=<github_user>
export GITHUB_TOKEN=<github_token>
make
make build-images
```
