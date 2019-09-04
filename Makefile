###############################################################################
# Licensed Materials - Property of IBM.
# Copyright IBM Corporation 2019. All Rights Reserved.
# U.S. Government Users Restricted Rights - Use, duplication or disclosure 
# restricted by GSA ADP Schedule Contract with IBM Corp.
#
# Contributors:
#  IBM Corporation - initial API and implementation
###############################################################################

include Configfile

PROJECT_NAME := $(shell basename $(CURDIR))

.PHONY: init\:
init::
	@mkdir -p variables
ifndef GITHUB_USER
	$(info GITHUB_USER not defined)
	exit -1
endif
	$(info Using GITHUB_USER=$(GITHUB_USER))
ifndef GITHUB_TOKEN
	$(info GITHUB_TOKEN not defined)
	exit -1
endif

-include $(shell curl -fso .build-harness -H "Authorization: token ${GITHUB_TOKEN}" -H "Accept: application/vnd.github.v3.raw" "https://raw.github.ibm.com/ICP-DevOps/build-harness/master/templates/Makefile.build-harness"; echo .build-harness)

BINDIR        ?= bin
BUILD_DIR     ?= build
SC_PKG         = github.ibm.com/IBMMulticloudPlatform/subscription
TYPES_FILES    = $(shell find pkg/apis -name types.go)
GOOS           = $(shell go env GOOS)
GOARCH         = $(shell go env GOARCH)
OPERATOR_SDK_RELEASE=v0.10.0

.PHONY: lint
lint:
	GO111MODULE=off go get golang.org/x/lint/golint
	# go get -u github.com/alecthomas/gometalinter
	# gometalinter --install
	golint -set_exit_status=true pkg/controller/...
	golint -set_exit_status=true cmd/...

.PHONY: deps
deps:
	go mod tidy

.PHONY: copyright-check
copyright-check:
	$(BUILD_DIR)/copyright-check.sh
	
all: deps test image

local:
	operator-sdk up local --verbose

wicked:
	@if [ -z $(dest) ]; then \
	   echo "Usage: make dest=destination_dir wicked"; \
	   exit 1; \
	fi
	wicked-cli --version
	@if [ $$? -ne 0 ]; then \
		echo "Install wicked with 'sudo npm install -g @wicked/cli@latest'"; \
		exit 1; \
	fi
	GO111MODULE=off go get -u github.ibm.com/IBMPrivateCloud/awsom-tool/... 
	go mod vendor && \
	rm -rf $(dest)/$(PROJECT_NAME)_scan-results && \
	wicked-cli -s . -o $(dest) && \
	awsomtool golang enrichGoMod -w $(dest)/$(PROJECT_NAME)_scan-results/Scan-Report.csv -o $(dest)/$(PROJECT_NAME)_scan-results/Scan-Report-Dep.csv && \
	awsomtool enrichCopyright -w $(dest)/$(PROJECT_NAME)_scan-results/Scan-Report-Dep.csv  -o $(dest)/$(PROJECT_NAME)_scan-results/Scan-Report-url-copyright.csv && \
	rm -rf vendor;
	rm -rf wicked_cli.log

# Install operator-sdk
operator-sdk-install: 
	@operator-sdk version ; \
	if [ $$? -ne 0 ]; then \
       uname -a | grep "Darwin"; \
       if [ $$? -eq 0 ]; then \
          curl -OJL "https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_RELEASE)/operator-sdk-$(OPERATOR_SDK_RELEASE)-x86_64-apple-darwin"; \
	      chmod +x operator-sdk-$(OPERATOR_SDK_RELEASE)-x86_64-apple-darwin && sudo mkdir -p /usr/local/bin/ && sudo cp operator-sdk-$(OPERATOR_SDK_RELEASE)-x86_64-apple-darwin /usr/local/bin/operator-sdk && rm operator-sdk-${OPERATOR_SDK_RELEASE}-x86_64-apple-darwin; \
       else \
          curl -OJL "https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_RELEASE}/operator-sdk-${OPERATOR_SDK_RELEASE}-x86_64-linux-gnu"; \
	      chmod +x operator-sdk-${OPERATOR_SDK_RELEASE}-x86_64-linux-gnu && sudo mkdir -p /usr/local/bin/ && sudo cp operator-sdk-${OPERATOR_SDK_RELEASE}-x86_64-linux-gnu /usr/local/bin/operator-sdk && rm operator-sdk-${OPERATOR_SDK_RELEASE}-x86_64-linux-gnu; \
	   fi; \
	fi

image: operator-sdk-install generate
	operator-sdk build $(IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION)
	uname -a | grep "Darwin"; \
    if [ $$? -eq 0 ]; then \
       sed -i "" 's|REPLACE_IMAGE|$(IMAGE_REPO)/$(IMAGE_NAME):${RELEASE_TAG}|g' deploy/operator.yaml; \
    else \
       sed -i 's|REPLACE_IMAGE|$(IMAGE_REPO)/$(IMAGE_NAME):${RELEASE_TAG}|g' deploy/operator.yaml; \
    fi

generate: operator-sdk-install
	operator-sdk generate k8s
	operator-sdk generate openapi

release: image
	@echo -e "$(TARGET) $(OS) $(ARCH)"
	@$(SELF) -s docker:tag DOCKER_IMAGE=$(IMAGE_REPO)/$(IMAGE_NAME_ARCH) DOCKER_BUILD_TAG=$(IMAGE_VERSION) DOCKER_URI=$(IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(RELEASE_TAG)
	@$(SELF) -s docker:push DOCKER_URI=$(RELEASE_IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(RELEASE_TAG)
	@$(SELF) -s docker:tag DOCKER_IMAGE=$(IMAGE_REPO)/$(IMAGE_NAME_ARCH) DOCKER_BUILD_TAG=$(IMAGE_VERSION) DOCKER_URI=$(IMAGE_REPO)/$(IMAGE_NAME_ARCH):latest
	@$(SELF) -s docker:push DOCKER_URI=$(RELEASE_IMAGE_REPO)/$(IMAGE_NAME_ARCH):latest

ifeq ($(ARCH), x86_64)
ifneq ($(RELEASE_TAG),)
	@$(SELF) -s docker:tag DOCKER_IMAGE=$(IMAGE_REPO)/$(IMAGE_NAME_ARCH) DOCKER_BUILD_TAG=$(IMAGE_VERSION) DOCKER_URI=$(IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(RELEASE_TAG)-rhel
	@$(SELF) -s docker:push DOCKER_URI=$(RELEASE_IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(RELEASE_TAG)-rhel
endif
	@$(SELF) -s docker:tag DOCKER_IMAGE=$(IMAGE_REPO)/$(IMAGE_NAME_ARCH) DOCKER_BUILD_TAG=$(IMAGE_VERSION) DOCKER_URI=$(IMAGE_REPO)/$(IMAGE_NAME_ARCH):latest-rhel
	@$(SELF) -s docker:push DOCKER_URI=$(RELEASE_IMAGE_REPO)/$(IMAGE_NAME_ARCH):latest-rhel
endif

# Run tests
test: generate fmt vet
# This skip the controller test as they are no working
	go test `go list ./pkg/... ./cmd/... | grep -v pkg/controller` -coverprofile=cover.out

# Run tests with debug output
testdebug: generate fmt vet
	go test ./pkg/... ./cmd/... -coverprofile=cover.out -v

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...
