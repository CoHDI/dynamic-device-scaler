PROJECT_NAME ?= dynamic-device-scaler
VERSION ?= v0.1.0
IMG ?= localhost/$(PROJECT_NAME):$(VERSION)

# Directories
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)
CONFIG_DIR ?= config
DEPLOY_DIR ?= deploy

# Tooling
GO ?= go
KUSTOMIZE_VERSION ?= v5.6.0
KUSTOMIZE ?= $(LOCALBIN)/kustomize
KUBECTL ?= kubectl

# --------------------------
# Build Targets
# --------------------------
.PHONY: all
all: build

.PHONY: build
build:
	$(GO) build -o $(LOCALBIN)/dynamic-device-scaler cmd/main.go

.PHONY: run
run:
	$(GO) run ./cmd/main.go

# --------------------------
# Testing Targets
# --------------------------
.PHONY: test
test:
	$(GO) test ./... -coverprofile=coverage.out

.PHONY: coverage
coverage: test
	$(GO) tool cover -html=coverage.out

# --------------------------
# Image Management
# --------------------------
.PHONY: docker-build
docker-build:
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push:
	docker push $(IMG)

# --------------------------
# Code Quality
# --------------------------
.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: lint
lint: fmt vet

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary

$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e ;\
echo "Downloading $(2)@$(3)" ;\
GOBIN=$(LOCALBIN) $(GO) install $(2)@$(3) ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef

# --------------------------
# Deployment Management
# --------------------------
.PHONY: manifests
manifests: kustomize
	@mkdir -p $(DEPLOY_DIR)
	$(KUSTOMIZE) build $(CONFIG_DIR)/default > $(DEPLOY_DIR)/manifests.yaml

.PHONY: deploy
deploy: manifests
	$(KUSTOMIZE) build $(CONFIG_DIR)/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: manifests
	$(KUSTOMIZE) build $(CONFIG_DIR)/default | $(KUBECTL) delete -f -

# --------------------------
# Utility Targets
# --------------------------
.PHONY: clean
clean:
	rm -rf $(LOCALBIN)
	rm -f coverage.out
	rm -f $(DEPLOY_DIR)

.PHONY: help
help:
	@echo "Operator Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build         - Build operator binary"
	@echo "  make run           - Run operator locally"
	@echo "  make test          - Run tests"
	@echo "  make coverage      - Generate test coverage report"
	@echo "  make docker-build  - Build operator docker image"
	@echo "  make docker-push   - Push operator docker image"
	@echo "  make fmt           - Format source code"
	@echo "  make vet           - Vet source code"
	@echo "  make lint          - Run all linting checks"
	@echo "  make manifests     - Generate deployment manifests"
	@echo "  make deploy        - Deploy operator to cluster"
	@echo "  make undeploy      - Undeploy operator from cluster"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make help          - Show this help"