############################################################################
# Usage: 
# 
# mkdir -p ~/.config/antithesis
# mv service-account.json ~/.config/antithesis
# export ANTITHESIS_GAR_KEY=$(cat ~/.config/antithesis/service-account.json) 
# make build_and_push_all
# ##########################################################################

.PHONY: login setup_buildx extract_metadata \
        build_and_push_config build_and_push_order build_and_push_payment build_and_push_test_template \
        build_and_push_all env

# Docker registry configuration. 

REGISTRY := us-central1-docker.pkg.dev/molten-verve-216720/ant-pdogfood-repository
VERSION := v1
DOCKER_PLATFORM := linux/amd64

# Git metadata for tags. 

GIT_SHA := $(shell git rev-parse --short HEAD)
DOCKER_BUILDX := docker buildx

# Image names with registry. 

CONFIG_IMAGE := $(REGISTRY)/config
ORDER_IMAGE := $(REGISTRY)/order
PAYMENT_IMAGE := $(REGISTRY)/payment
TEST_TEMPLATE_IMAGE := $(REGISTRY)/test-template

# Tags for each image. 

CONFIG_TAGS := $(CONFIG_IMAGE):$(GIT_SHA) $(CONFIG_IMAGE):latest
ORDER_TAGS := $(ORDER_IMAGE):$(GIT_SHA) $(ORDER_IMAGE):latest
PAYMENT_TAGS := $(PAYMENT_IMAGE):$(GIT_SHA) $(PAYMENT_IMAGE):latest
TEST_TEMPLATE_TAGS := $(TEST_TEMPLATE_IMAGE):$(GIT_SHA) $(TEST_TEMPLATE_IMAGE):latest

# Docker login. 

login:
	@echo "Logging into Google Artifact Registry..."
	@docker login -u _json_key -p "$$ANTITHESIS_GAR_KEY" $(REGISTRY)

# Setup buildx builder. 

setup_buildx:
	@echo "Enabling BuildKit and setting up buildx builder..."
	@if ! docker buildx inspect mybuilder > /dev/null 2>&1; then \
		docker buildx create --name mybuilder --use; \
	fi
	docker buildx inspect --bootstrap
	docker run --privileged --rm tonistiigi/binfmt --install all

# Extract metadata (similar to GitHub Actions metadata-action). 

extract_metadata:
	@echo "Extracting metadata for images..."
	@echo "Git SHA: $(GIT_SHA)"
	@echo "Registry: $(REGISTRY)"

# Build and push commands using buildx. 

build_and_push_config:
	$(DOCKER_BUILDX) build \
		--platform $(DOCKER_PLATFORM) \
		-t $(CONFIG_IMAGE):$(GIT_SHA) \
		-t $(CONFIG_IMAGE):latest \
		-f config/Dockerfile ./config \
		--push=true

build_and_push_order:
	$(DOCKER_BUILDX) build \
		--platform $(DOCKER_PLATFORM) \
		-t $(ORDER_IMAGE):$(GIT_SHA) \
		-t $(ORDER_IMAGE):latest \
		-f orderService/Dockerfile ./orderService \
		--push=true

build_and_push_payment:
	$(DOCKER_BUILDX) build \
		--platform $(DOCKER_PLATFORM) \
		-t $(PAYMENT_IMAGE):$(GIT_SHA) \
		-t $(PAYMENT_IMAGE):latest \
		-f paymentService/Dockerfile ./paymentService \
		--push=true

build_and_push_test_template:
	$(DOCKER_BUILDX) build \
		--platform $(DOCKER_PLATFORM) \
		-t $(TEST_TEMPLATE_IMAGE):$(GIT_SHA) \
		-t $(TEST_TEMPLATE_IMAGE):latest \
		-f test/opt/antithesis/test/v1/Dockerfile ./test/opt/antithesis/test/v1/ \
		--push=true

# Grouped commands.

build_and_push_all: extract_metadata build_and_push_config build_and_push_order build_and_push_payment build_and_push_test_template

env: login setup_buildx build_and_push_all
	@echo "All images have been built and pushed successfully"
