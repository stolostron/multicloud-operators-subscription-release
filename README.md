# multicloud-operators-subscription-release

[![Build](https://travis-ci.com/open-cluster-management/multicloud-operators-subscription-release.svg?branch=master)](https://travis-ci.com/open-cluster-management/multicloud-operators-subscription-release.svg?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/open-cluster-management/multicloud-operators-subscription-release)](https://goreportcard.com/report/github.com/open-cluster-management/multicloud-operators-subscription-release)
[![GoDoc](https://godoc.org/github.com/open-cluster-management/multicloud-operators-subscription-release?status.svg)](https://godoc.org/github.com/open-cluster-management/multicloud-operators-subscription-release?status.svg)
[![Sonarcloud Status](https://sonarcloud.io/api/project_badges/measure?project=open-cluster-management_multicloud-operators-subscription-release&metric=coverage)](https://sonarcloud.io/api/project_badges/measure?project=open-cluster-management_multicloud-operators-subscription-release&metric=coverage)
[![License](https://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [multicloud-operators-subscription-release](#multicloud-operators-subscription-release)
    - [What is the multicloud-operators-subscription-release](#what-is-the-multicloud-operators-subscription-release)
    - [Community, discussion, contribution, and support](#community-discussion-contribution-and-support)
    - [Getting Started](#getting-started)
        - [Prerequisites](#prerequisites)
        - [Deployment](#deployment)
    - [Security Response](#security-response)
    - [References](#references)
        - [multicloud-operators repositories](#multicloud-operators-repositories)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## What is the multicloud-operators-subscription-release

The multicloud-operators-subscription-release is composed of the helmrelease controller which will manage the helmrelease CR, download the chart from the helmrepo or github and then call the operator-sdk helm-operator reconcile to start the deployment of each chart.

## Community, discussion, contribution, and support

Check the [CONTRIBUTING Doc](CONTRIBUTING.md) for how to contribute to the repo.

------

## Getting Started

### Prerequisites

Check the [Development Doc](docs/development.md) for how to contribute to the repo.

### Deployment

Check the [Deployment Doc](docs/deployment.md) for how to deploy the operator.

## Security Response

Check the [Security Doc](SECURITY.md) if you've found a security issue.

## References

### multicloud-operators repositories

- [multicloud-operators-application](https://github.com/open-cluster-management/multicloud-operators-application)
- [multicloud-operators-channel](https://github.com/open-cluster-management/multicloud-operators-channel)
- [multicloud-operators-deployable](https://github.com/open-cluster-management/multicloud-operators-deployable)
- [multicloud-operators-placementrule](https://github.com/open-cluster-management/multicloud-operators-placementrule)
- [multicloud-operators-subscription](https://github.com/open-cluster-management/multicloud-operators-subscription)
- [multicloud-operators-subscription-release](https://github.com/open-cluster-management/multicloud-operators-subscription-release)
