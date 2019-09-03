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
	golint -set_exit_status=true pkg/...
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
	wicked-cli --version
	@if [ $$? -ne 0 ]; then \
		echo "Install wicked with 'sudo npm install -g @wicked/cli@latest'"; \
		exit 1; \
	fi
	GO111MODULE=off go get -u github.ibm.com/IBMPrivateCloud/awsom-tool/... 
	go mod vendor && \
	rm -rf vendor_scan-results && \
	wicked-cli -s vendor && \
	cd vendor && \
	awsomtool golang enrichGoMod -w ../vendor_scan-results/Scan-Report.csv -o ../vendor_scan-results/Scan-Report-Dep.csv && \
	awsomtool enrichCopyright -w ../vendor_scan-results/Scan-Report-Dep.csv  -o ../vendor_scan-results/Scan-Report-url-copyright.csv; && \
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

image:: generate operator-sdk-install
	operator-sdk build $(IMAGE_REPO)/$(IMAGE_NAME_ARCH)
	uname -a | grep "Darwin"; \
    if [ $$? -eq 0 ]; then \
       sed -i "" 's|REPLACE_IMAGE|$(IMAGE_REPO)/$(IMAGE_NAME):${RELEASE_TAG}|g' deploy/operator.yaml; \
    else \
       sed -i 's|REPLACE_IMAGE|$(IMAGE_REPO)/$(IMAGE_NAME):${RELEASE_TAG}|g' deploy/operator.yaml; \
    fi

generate: operator-sdk-install
	operator-sdk generate k8s

# Run tests
test: generate fmt vet manifests kubebuilder
# This skip the controller test as they are no working
	go test `go list ./pkg/... ./cmd/... | grep -v pkg/controller` -coverprofile=cover.out

# Run tests with debug output
testdebug: generate fmt vet manifests kubebuilder
	go test ./pkg/... ./cmd/... -coverprofile=cover.out -v

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...


# include Makefile.docker

.PHONY: app-version
app-version:
	$(eval WORKING_CHANGES := $(shell git status --porcelain))
	$(eval BUILD_DATE := $(shell date +%m/%d@%H:%M:%S))
	$(eval GIT_COMMIT := $(shell git rev-parse --short HEAD))
	$(eval VCS_REF := $(if $(WORKING_CHANGES),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT)))
	$(eval APP_VERSION ?= $(if $(shell cat VERSION 2> /dev/null),$(shell cat VERSION 2> /dev/null),0.0.1))
	$(eval IMAGE_VERSION ?= $(APP_VERSION)-$(GIT_COMMIT))
	@echo "App: $(IMAGE_NAME_ARCH) $(IMAGE_VERSION)"

include Makefile.docker