# multicloud-operators-subscription-release

<p align="center"><a href="http://35.227.205.240/?job=build_multicloud-operators-subscription-release_postsubmit">
<!-- prow build badge, godoc, and go report card-->
<img alt="Build Status" src="http://35.227.205.240/badge.svg?jobs=build_multicloud-operators-subscription-release_postsubmit">
</a> <a href="https://godoc.org/github.com/IBM/multicloud-operators-subscription-release"><img src="https://godoc.org/github.com/IBM/multicloud-operators-subscription-release?status.svg"></a> <a href="https://goreportcard.com/report/github.com/IBM/multicloud-operators-subscription-release"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/IBM/multicloud-operators-subscription-release" /></a> <a href="https://codecov.io/github/IBM/multicloud-operators-subscription-release?branch=master"><img alt="Code Coverage" src="https://codecov.io/gh/IBM/multicloud-operators-subscription-release/branch/master/graphs/badge.svg?branch=master" /></a></p>

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [multicloud-operators-subscription-release](#multicloud-operators-subscription-release)
    - [What is the multicloud-operators-subscription-release](#what-is-the-multicloud-operators-subscription-release)
    - [Community, discussion, contribution, and support](#community-discussion-contribution-and-support)
    - [Getting Started](#getting-started)
        - [Prerequisites](#prerequisites)
        - [Deployment](#deployment)
        - [Trouble shooting](#trouble-shooting)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## What is the multicloud-operators-subscription-release

The multicloud-operators-subscription-release is composed of 2 controllers. The helmchartsubscription controller which is in charge of managing the helmchartsubscription CR. That CR defines the location of the charts (helmrepo or github) and filters to select a subset of charts to deploy. the helmchartsubscription controller will then create a number of helmreleases and these are managed by the helmrelease controller. The helmrelease controller will manage the helmrelease CR, download the chart from the helmrepo or github and then call the operator-sdk helm-operator methods to start the deployment of each chart.

The helmrelease controller can be use independently without the helmchartsubscription controller. The flag `--helmchart-subscription-controller-disabled` can be used to disable the helmchartsubscription controller.

## Community, discussion, contribution, and support

Check the [CONTRIBUTING Doc](CONTRIBUTING.md) for how to contribute to the repo.

------

## Getting Started

### Prerequisites

Check the [Development Doc](docs/development.md) for how to contribute to the repo.

### Deployment

Check the [Deployment Doc](docs/deployment.md) for how to deploy the multicloud-operators-subscription-release in standalone mode.

### Trouble shooting

Please refer to [Trouble shooting documentation](docs/trouble_shooting.md) for further info.
