# Image URL to use all building/pushing image targets
IMG ?= aveshasystems/kubeslice-controller:latest


# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec


## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.17.3
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION ?= 1.26.0

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=controller-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: generate-events
generate-events:
	go run hack/events/generate/generate.go
	go fmt ./...

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet -composites=false ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./controllers/controller/... -coverprofile cover.out

.PHONY: test-local
test-local: envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./controllers/controller/... -coverprofile cover.out

.PHONY: int-test
int-test: envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./controllers/controller/... -coverprofile cover.out

.PHONY: generate-yamls
generate-yamls: manifests kustomize ## Generates the yaml files
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default


##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker buildx create --name container --driver=docker-container || true
	docker build --builder container --platform linux/amd64,linux/arm64 -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker buildx create --name container --driver=docker-container || true
	docker build --push --builder container --platform linux/amd64,linux/arm64 -t ${IMG} .

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies


.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)


KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }


.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

apply-sample: ## Generate sample manifests.
	kubectl apply -f config/samples/controller_v1alpha1_project.yaml
	kubectl apply -f config/samples/controller_v1alpha1_cluster.yaml -n=kubeslice-cisco
	kubectl apply -f config/samples/controller_v1alpha1_sliceconfig.yaml -n=kubeslice-cisco
	kubectl apply -f config/samples/controller_v1alpha1_serviceexportconfig.yaml -n=kubeslice-cisco

.PHONY: webhookCA
webhookCA: ## Install the Cert Manager for Admission controller Webhooks.
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.7.0/cert-manager.yaml

.PHONY: cleanup
cleanup:
	kubectl apply -f config/cleanup/cleanup_job.yaml -n=kubeslice-controller

.PHONY: generate-mocks
generate-mocks: ## Generate mocks for the controller-runtime.
	mockery --dir service/ --all  --output service/mocks
	mockery --dir util/ --name Client  --output util/mocks
	mockery --dir metrics/ --all --output metrics/mocks

.PHONY: unit-test
unit-test: ## Run local unit tests.
	go test -gcflags=-l ./service --coverprofile=coverage.out
	mkdir -p coverage-report
	go tool cover -html=coverage.out -o coverage-report/report.html

.PHONY: unit-test-docker
unit-test-docker: ## Run local unit tests in a docker container.
	docker build -f unit_tests.dockerfile -o . .

.PHONY: chart-deploy
chart-deploy:
	## Deploy the artifacts using helm
	## Usage: make chart-deploy VALUESFILE=[valuesfilename]
	helm upgrade --install cert-manager avesha/cert-manager --namespace cert-manager --create-namespace --set installCRDs=true
	helm upgrade --install kubeslice-controller -n kubeslice-controller avesha/kubeslice-controller -f ${VALUESFILE} --create-namespace

.PHONY: chart-undeploy
chart-undeploy:
	helm uninstall kubeslice-controller -n kubeslice-controller
	helm uninstall cert-manager -n cert-manager

.PHONY: chart-deploy-cert
chart-deploy-cert:
	## Deploy the artifacts using helm
	helm upgrade --install cert-manager avesha/cert-manager --namespace cert-manager --create-namespace --set installCRDs=true

.PHONY: chart-deploy-controller
chart-deploy-controller:
	## Deploy the artifacts using helm
    ## Usage: make chart-deploy VALUESFILE=[valuesfilename]
	helm upgrade --install kubeslice-controller -n kubeslice-controller avesha/kubeslice-controller -f ${VALUESFILE} --create-namespace
