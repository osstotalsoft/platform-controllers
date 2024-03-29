################################################################################
# Variables                                                                    #
################################################################################

OUT_DIR := ./dist
GIT_COMMIT  = $(shell git rev-list -1 HEAD)
GIT_VERSION = $(shell git describe --always --abbrev=7 --dirty)
VERSION ?= edge

# Helm template and install setting
HELM:=helm
RELEASE_NAME?=platform-controllers
HELM_NAMESPACE?=default
HELM_CHART_ROOT:=./helm

################################################################################
# Go build details                                                             #
################################################################################
BASE_PACKAGE_NAME := totalsoft.ro/platform-controllers

DEFAULT_LDFLAGS:=-X $(BASE_PACKAGE_NAME)/internal/version.gitcommit=$(GIT_COMMIT) \
  -X $(BASE_PACKAGE_NAME)/internal/version.gitversion=$(GIT_VERSION) \
  -X $(BASE_PACKAGE_NAME)/internal/version.version=$(VERSION)

################################################################################
# Target: build-linux                                                          #
################################################################################
build-linux:
	mkdir -p $(OUT_DIR)
	CGO_ENABLED=0 GOOS=linux go build -o $(OUT_DIR) -ldflags "$(DEFAULT_LDFLAGS) -s -w" ./cmd/tenant-provisioner ./cmd/platform-controller ./cmd/configuration-domain-controller

modtidy:
	go mod tidy

upgrade-all:
	go get -u ./...
	go mod tidy

init-proto:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

gen-proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative internal/messaging/rusi/v1/rusi.proto

test:
	CGO_ENABLED=0 go test -v `go list ./... | grep -v 'platform-controllers/pkg/generated'`

include hack/generate_kube_crd.mk
include docker/docker.mk
include internal/controllers/provisioning/provisioners/pulumi/fluxcd/generate_pulumi_fluxcd_apis.mk
