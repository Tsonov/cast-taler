APP ?= echo
REPOSITORY ?= ghcr.io/tsonov/cast-taler
TAG ?= latest
ORGANIZATION_ID ?=
CLUSTER_ID ?=
CASTAI_API_TOKEN ?=
CASTAI_API_URI ?= api.dev-master.cast.ai
BUOYANT_LICENSE ?=

include Makefile.vars

.PHONY: build-push
build-push:
	docker build -t $(REPOSITORY)/$(APP):$(TAG) .
	docker push $(REPOSITORY)/$(APP):$(TAG)

.PHONY: build-push-optimizer
build-push-optimizer:
	docker build -t $(REPOSITORY):optimizer -f optimizer.Dockerfile .
	docker push $(REPOSITORY):optimizer

.PHONY: deploy-optimizer
deploy-optimizer: create-namespace build-push-optimizer
	@echo "→ Deploying optimizer with REPOSITORY=$(REPOSITORY)"
	@export OPTIMIZER_IMAGE=$(REPOSITORY):optimizer && \
	export BUOYANT_LICENSE=$(BUOYANT_LICENSE) && \
	export CASTAI_API_URI=$(CASTAI_API_URI) && \
	export ORGANIZATION_ID=$(ORGANIZATION_ID) && \
	export CLUSTER_ID=$(CLUSTER_ID) && \
	export CASTAI_API_TOKEN=$(CASTAI_API_TOKEN) && \
	cat ./hack/optimizer/deployment.yaml | envsubst '$$OPTIMIZER_IMAGE $$BUOYANT_LICENSE $$CASTAI_API_URI $$ORGANIZATION_ID $$CLUSTER_ID $$CASTAI_API_TOKEN' | kubectl apply -f -

.PHONY: create-namespace
create-namespace:
	kubectl create namespace taler --dry-run=client -o yaml | kubectl apply -f -

.PHONY: deploy
deploy-app: create-namespace build-push
	@echo "→ Deploying with TAG=$(TAG)"
	@export ECHO_TAG=$(TAG) && \
	kubectl kustomize ./hack/app/ \
	  | envsubst '$$ECHO_TAG' \
	  | kubectl apply -f -


.PHONY: deploy-observability
deploy-observability: create-namespace
	kubectl kustomize ./hack/observability/ | \
      kubectl apply -f -

.PHONY: deploy
deploy: deploy-app deploy-observability deploy-optimizer

.PHONY: connect-observability
connect-observability:
	kubectl port-forward -n taler svc/observability-service 3000:3000

.PHONY: destroy
destroy:
	kubectl delete --ignore-not-found namespace taler
	kubectl delete --ignore-not-found -f ./hack/app/traffic-app.yaml
	kubectl delete --ignore-not-found namespace linkerd
	kubectl delete --ignore-not-found -f ./hack/optimizer/deployment.yaml

.PHONY: linkerd-install
linkerd-install:
	BUOYANT_LICENSE=$(BUOYANT_LICENSE) ./hack/linkerd/install.sh

.PHONY: linkerd-uninstall
linkerd-uninstall:
	BUOYANT_LICENSE=$(BUOYANT_LICENSE) ./hack/linkerd/uninstall.sh

.PHONY: hazl-enable
hazl-enable: apply-pod-mutation
	./hack/linkerd/hazl-enable.sh

.PHONY: hazl-disable
hazl-disable: remove-pod-mutation
	./hack/linkerd/hazl-disable.sh

.PHONY: remove-pod-mutation
remove-pod-mutation:
	ORGANIZATION_ID=$(ORGANIZATION_ID) CLUSTER_ID=${CLUSTER_ID} CASTAI_API_TOKEN=${CASTAI_API_TOKEN} CASTAI_API_URI=${CASTAI_API_URI} ./hack/linkerd/pod-mutator.sh remove

.PHONY: apply-pod-mutation
apply-pod-mutation:
	ORGANIZATION_ID=$(ORGANIZATION_ID) CLUSTER_ID=${CLUSTER_ID} CASTAI_API_TOKEN=${CASTAI_API_TOKEN} CASTAI_API_URI=${CASTAI_API_URI} ./hack/linkerd/pod-mutator.sh

.PHONY: apply-topologyspread-pod-mutation
apply-topologyspread-pod-mutation:
	ORGANIZATION_ID=$(ORGANIZATION_ID) CLUSTER_ID=${CLUSTER_ID} CASTAI_API_TOKEN=${CASTAI_API_TOKEN} CASTAI_API_URI=${CASTAI_API_URI} ./hack/topologyspread/pod-mutator.sh
