#!/bin/bash

#
# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and# limitations under the License.


###!!!!!!!! On travis this script is run on the .git level
set -e
echo "E2E TESTS GO HERE!"

# need to find a way to use the Makefile to set these
REGISTRY=quay.io/open-cluster-management
IMG=$(cat COMPONENT_NAME 2> /dev/null)
IMAGE_NAME=${REGISTRY}/${IMG}
BUILD_IMAGE=${IMAGE_NAME}:latest
OPERATOR_NAME=multicluster-operators-subscription-release

if [ "$TRAVIS_BUILD" != 1 ]; then
    echo -e "Build is on Travis"

    # Download and install kubectl
    echo -e "\nGet kubectl binary\n"
    PLATFORM=`uname -s | awk '{print tolower($0)}'`
    if [ "`which kubectl`" ]; then
        echo "kubectl PATH is `which kubectl`"
    else
        mkdir -p $(pwd)/bin
        curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/$PLATFORM/amd64/kubectl && mv kubectl $(pwd)/bin/ && chmod +x $(pwd)/bin/kubectl
        export PATH=$PATH:$(pwd)/bin
        echo "kubectl PATH is `which kubectl`"
    fi
    COMPONENT_VERSION=$(cat COMPONENT_VERSION 2> /dev/null)
    BUILD_IMAGE=${IMAGE_NAME}:${COMPONENT_VERSION}${COMPONENT_TAG_EXTENSION}

    echo -e "\nBUILD_IMAGE tag $BUILD_IMAGE\n"
    echo -e "Modify deployment to point to the PR image\n"
    sed -i -e "s|image: .*:latest$|image: $BUILD_IMAGE|" deploy/operator.yaml

    echo -e "\nDownload and install KinD\n"
    GO111MODULE=on go get sigs.k8s.io/kind
else
    echo -e "\nBuild is on Local ENV, will delete the API container first\n"
    docker kill e2e || true
fi

kind delete cluster
if [ $? != 0 ]; then
        exit $?;
fi

kind create cluster
if [ $? != 0 ]; then
        exit $?;
fi

sleep 15


echo -e "\nPath for container in YAML $(grep 'image: .*' deploy/operator.yaml)\n"

echo -e "\nLoad build image ($BUILD_IMAGE)to kind cluster\n"
kind load docker-image $BUILD_IMAGE
if [ $? != 0 ]; then
    exit $?;
fi

echo -e "\nSwitch kubeconfig to kind cluster\n"
kubectl cluster-info --context kind-kind

echo -e "\nApplying operator to kind cluster\n"
kubectl apply -f deploy
if [ $? != 0 ]; then
    exit $?;
fi


echo -e "\nApply CRDs\n"
kubectl apply -f deploy/crds

if [ "$TRAVIS_BUILD" != 1 ]; then
    echo -e "\nWait for pod to be ready\n"
    sleep 35
fi

echo -e "\nCheck if subscription-release operator is created\n"
kubectl rollout status deployment/multicluster-operators-subscription-release
if [ $? != 0 ]; then
    echo "failed to deploy the subscription operator"
    exit $?;
fi

echo -e "\nApply the Apache service with basic auth and helm chart\n"
kubectl apply -f apache-basic-auth/apache-basic-auth-service.yaml

if [ "$TRAVIS_BUILD" != 1 ]; then
    echo -e "\nWait for Apache pod to be ready\n"
    sleep 35
fi

echo -e "\nRun API test server\n"
mkdir -p cluster_config
kind get kubeconfig > cluster_config/hub

# over here, we are using test server binary and run it on the host instead of
# run the server in a docker container is due to the fact that, on macOS it's
# very hard to map the host network to container

# over here, we are build the test server on the fly since, the `go get` will
# mess up the go.mod file when doing the local test

# Only latest tag with format "v0~9*.0~9*.0~9*" is picked up
LATEST_TAG=`git ls-remote --tags https://github.com/open-cluster-management/applifecycle-backend-e2e.git| awk '{print $2}' | sed 's$refs/tags/$$' | grep "v[0-9]*\.[0-9]*\.[0-9]*$" | sort -nr | head -n1`
echo -e "\nGet latest version tag of applifecycle-backend-e2e: $LATEST_TAG"
echo -e "\nGet the applifecycle-backend-e2e data"
go get github.com/open-cluster-management/applifecycle-backend-e2e@$LATEST_TAG

export PATH=$PATH:~/go/bin
E2E_BINARY_NAME="applifecycle-backend-e2e"

echo -e "\nTerminate the running test server\n"
E2E_PS=$(ps aux | grep ${E2E_BINARY_NAME} | grep -v 'grep' | awk '{print $2}')
if [ "$E2E_PS" != "" ]; then
    kill -9 $E2E_PS
fi

${E2E_BINARY_NAME} -cfg cluster_config &

sleep 10

function cleanup()
{
    echo -e "\nTerminate the running test server\n"
	ps aux | grep ${E2E_BINARY_NAME} | grep -v 'grep' | awk '{print $2}' | xargs kill -9
   	echo -e "\nRunning images\n"
	kubectl get deploy -A -o jsonpath='{.items[*].spec.template.spec.containers[*].image}' | xargs -n1 echo

	kubectl get po -A
}

trap cleanup EXIT

echo -e "\nStart to run e2e test(s)\n"
go test -v ./e2e -timeout 30m

exit 0;
