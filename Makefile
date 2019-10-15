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

# .PHONY: init\:
# init::
# 	@mkdir -p variables
# ifndef GITHUB_USER
# 	$(info GITHUB_USER not defined)
# 	exit -1
# endif
# 	$(info Using GITHUB_USER=$(GITHUB_USER))
# ifndef GITHUB_TOKEN
# 	$(info GITHUB_TOKEN not defined)
# 	exit -1
# endif

# -include $(shell curl -fso .build-harness -H "Authorization: token ${GITHUB_TOKEN}" -H "Accept: application/vnd.github.v3.raw" "https://raw.github.ibm.com/ICP-DevOps/build-harness/master/templates/Makefile.build-harness"; echo .build-harness)

BINDIR        ?= bin
BUILD_DIR     ?= build
SC_PKG         = github.com/IBM/multicloud-operators-subscription-release/subscription
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

ossc:
	@if [ -z $(dest) ]; then \
	   echo "Usage: make dest=destination_dir wicked"; \
	   exit 1; \
	fi
	rm -rf /tmp/awsom-tool; mkdir -p /tmp/awsom-tool; cd /tmp; git clone https://github.ibm.com/IBMPrivateCloud/awsom-tool --depth 1; cd awsom-tool; make local; cd $(CURDIR)
	rm -rf $(dest)/$(PROJECT_NAME)_scan-results && \
	mkdir -p $(dest)/$(PROJECT_NAME)_scan-results && \
	/tmp/awsom-tool/_build/awsomtool golang scan -o $(dest)/$(PROJECT_NAME)_scan-results/Scan-Report.csv && \
	/tmp/awsom-tool/_build/awsomtool enrichCopyright -w $(dest)/$(PROJECT_NAME)_scan-results/Scan-Report.csv  -o $(dest)/$(PROJECT_NAME)_scan-results/Scan-Report-url-copyright.csv && \
	rm -rf wicked_cli.log

check-licenses:
	@rm -rf /tmp/awsom-tool; mkdir -p /tmp/awsom-tool; cd /tmp; git clone https://github.ibm.com/IBMPrivateCloud/awsom-tool --depth 1; cd awsom-tool; make local; cd $(CURDIR)
	@$(eval RESULT = $(shell /tmp/awsom-tool/_build/awsomtool golang licenses -p .*GPL.* --format '{{.Path}} {{.LicenseType}}'))
	@if [ "$(RESULT)" != "" ]; then \
		echo "A License file contains the GPL word"; \
		echo -e $(RESULT); \
		exit 1; \
	fi

# Install operator-sdk
operator-sdk-install: 
	@operator-sdk version ; \
	if [ $$? -ne 0 ]; then \
       ./build/install-operator-sdk.sh; \
	fi

image: operator-sdk-install generate
	$(info Building operator)
	$(info --IMAGE: $(DOCKER_IMAGE))
	$(info --TAG: $(DOCKER_BUILD_TAG))
	operator-sdk build $(IMAGE_REPO)/$(IMAGE_NAME_ARCH):$(IMAGE_VERSION) --image-build-args "$(DOCKER_BUILD_OPTS)"	
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

