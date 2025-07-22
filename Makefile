APP ?= dummy
REPOSITORY ?= ghcr.io/tsonov/cast-taler

.PHONY: build-push
build-push:
	docker build -t $(REPOSITORY)/$(APP):latest .
	docker push $(REPOSITORY)/$(APP):latest
