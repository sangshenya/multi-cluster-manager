VERSION ?= $(shell git show -s --pretty=format:%H)
CORE_IMG ?= stellaris-core:$(VERSION)
PROXY_IMG ?= stellaris-proxy:$(VERSION)

REGISTRY ?= docker.io/harmonycloud
#REGISTRY ?= docker.io/sangshen

.PHONY: generate
generate: controller-gen
	./hack/code-generator/update-codegen.sh
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=kube/crd/bases
	cp $(shell pwd)/kube/crd/bases/* $(shell pwd)/charts/stellaris-core/crds/
	cp $(shell pwd)/kube/crd/bases/multicluster.harmonycloud.cn_clusterresources.yaml $(shell pwd)/charts/stellaris-proxy/crds/
	cp $(shell pwd)/kube/crd/bases/multicluster.harmonycloud.cn_resourceaggregatepolicies.yaml $(shell pwd)/charts/stellaris-proxy/crds/
	cp $(shell pwd)/kube/crd/bases/multicluster.harmonycloud.cn_multiclusterresourceaggregaterules.yaml $(shell pwd)/charts/stellaris-proxy/crds/

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

.PHONY: push
push: build
	docker tag $(CORE_IMG) $(REGISTRY)/$(CORE_IMG)
	docker tag $(PROXY_IMG) $(REGISTRY)/$(PROXY_IMG)
	docker push $(REGISTRY)/$(CORE_IMG)
	docker push $(REGISTRY)/$(PROXY_IMG)

.PHONY: build
build: build-core build-proxy

.PHONY: build-core
build-core:
	docker build -f build/Dockerfile-core -t $(CORE_IMG) .

.PHONY: build-proxy
build-proxy:
	docker build -f build/Dockerfile-proxy -t $(PROXY_IMG) .