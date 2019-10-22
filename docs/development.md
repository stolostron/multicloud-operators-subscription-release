# Development Guide

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Development Guide](#development-guide)
    - [Environment variable](#environment-variable)
    - [Launch Dev mode](#launch-dev-mode)
    - [Build](#build)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Environment variable

The environment variable `CHARTS_DIR` must be set when developing, it specifies the directory where the charts will be downloaded and expanded.

## Launch Dev mode

```bash
operator-sdk up local --verbose [--operator-flags "--helmchart-subscription-controller-disabled"]
```

## Build

operator-sdk build ibm/multicloud-operators-subscription-release:latest
docker tag ibm/multicloud-operators-subscription-release:latest mycluster.icp:8500/kube-system/ibm/multicloud-operators-subscription-release:latest
docker push mycluster.icp:8500/kube-system/ibm/multicloud-operators-subscription-release:latest
