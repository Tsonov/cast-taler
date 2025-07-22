APP ?= dummy
REPOSITORY ?= ghcr.io/tsonov/cast-taler

.PHONY: build-push
build-push:
	docker build -t $(REPOSITORY)/$(APP):latest .
	docker push $(REPOSITORY)/$(APP):latest

.PHONY: deploy-observability
deploy-observability:
	kubectl kustomize ./hack/observability/ | \
      kubectl apply -f -

.PHONY: connect-observability
connect-observability:
	kubectl port-forward -n taler svc/observability-service 3000:3000