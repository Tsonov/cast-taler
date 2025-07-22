APP ?= echo
REPOSITORY ?= ghcr.io/tsonov/cast-taler
TAG ?= latest

.PHONY: build-push
build-push:
	docker build -t $(REPOSITORY)/$(APP):$(TAG) .
	docker push $(REPOSITORY)/$(APP):$(TAG)

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
deploy: deploy-app deploy-observability

.PHONY: connect-observability
connect-observability:
	kubectl port-forward -n taler svc/observability-service 3000:3000

.PHONY: destroy
destroy:
	kubectl delete --ignore-not-found namespace taler
	kubectl delete --ignore-not-found -f ./hack/app/traffic-app.yaml