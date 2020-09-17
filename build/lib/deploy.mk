# ==============================================================================
# Makefile helper functions for deploy to developer env
#

KUBECTL := kubectl
NAMESPACE ?= dev

TIMESTAMP := $(shell /bin/date "+%Y-%m-%d---%H-%M-%S")

# Determine deploy files by looking into hack/deploy/*.yaml
DEPLOY_FILES=$(wildcard ${ROOT_DIR}/build/deploy/*.yaml)
# Determine deploy names by stripping out the dir names
DEPLOYS=$(foreach deploy,${DEPLOY_FILES},$(subst .yaml,,$(notdir ${deploy})))

.PHONY: deploy.run.all
deploy.run.all:
	@echo "===========> Deploying etcd"
	@cat $(ROOT_DIR)/build/deploy/etcd.yaml \
	 | sed "s/{{NAMESPACE}}/$(NAMESPACE)/g" \
	 | kubectl apply -f -
	@echo "===========> Deploying configmap"
	@cat $(ROOT_DIR)/build/deploy/configmap.yaml \
	 | sed "s/{{NAMESPACE}}/$(NAMESPACE)/g" \
	 | kubectl apply -f -
	@$(MAKE) deploy.run

.PHONY: deploy.run
deploy.run: $(addprefix deploy.run., $(DEPLOYS))

.PHONY: deploy.run.%
deploy.run.%:
	@echo "===========> Deploying $* $(VERSION)"
	@cat $(ROOT_DIR)/build/deploy/$*.yaml \
    	 | sed "s/{{NAMESPACE}}/$(NAMESPACE)/g" \
    	 | sed "s#{{REGISTRY_PREFIX}}#$(REGISTRY_PREFIX)#g" \
    	 | sed "s/{{VERSION}}/$(VERSION)/g" \
    	 | sed "s/{{TIMESTAMP}}/$(TIMESTAMP)/g" \
    	 | kubectl apply -f -