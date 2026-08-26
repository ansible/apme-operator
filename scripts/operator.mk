# Dev inner-loop variables for the APME operator.
# Usage: make -f scripts/operator.mk deploy   or include from the root Makefile.

NAMESPACE ?= apme
QUAY_USER ?= $(USER)
IMAGE ?= quay.io/$(QUAY_USER)/apme-operator:dev
DEV_CR ?= config/samples/apme_v1alpha1_apme.yaml
IMG ?= $(IMAGE)

.PHONY: deploy down lint

deploy:
	$(MAKE) -C $(CURDIR) docker-build docker-push deploy IMG=$(IMG)
	kubectl get ns $(NAMESPACE) >/dev/null 2>&1 || kubectl create ns $(NAMESPACE)
	kubectl apply -n $(NAMESPACE) -f $(DEV_CR)

down:
	kubectl delete -n $(NAMESPACE) -f $(DEV_CR) --ignore-not-found
	$(MAKE) -C $(CURDIR) undeploy

lint:
	$(MAKE) -C $(CURDIR) lint
