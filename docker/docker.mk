# Docker image build and push setting
DOCKER:=docker
DOCKERFILE_DIR?=./docker

# build docker image for linux
BIN_PATH=$(OUT_DIR)
DOCKERFILE:=Dockerfile

check-docker-env:
ifeq ($(DOCKER_REGISTRY),)
	$(error DOCKER_REGISTRY environment variable must be set)
endif
ifeq ($(DOCKER_TAG),)
	$(error DOCKER_TAG environment variable must be set)
endif

docker-build: check-docker-env
	$(DOCKER) build --build-arg PKG_FILES=* -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH)/. -t $(DOCKER_REGISTRY)/$(RELEASE_NAME):$(DOCKER_TAG)

docker-push: check-docker-env
	$(DOCKER) push $(DOCKER_REGISTRY)/$(RELEASE_NAME):$(DOCKER_TAG)

docker-all: docker-build docker-push
