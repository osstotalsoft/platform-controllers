#!/usr/bin/env bash

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODE_GENERATOR_DIR=~/go/pkg/mod/k8s.io/code-generator@v0.28.2
source "${CODE_GENERATOR_DIR}/kube_codegen.sh"

kube::codegen::gen_helpers \
    --input-pkg-root totalsoft.ro/platform-controllers/pkg/apis \
    --output-base "./../.." \
    --boilerplate "./hack/boilerplate.go.txt"

kube::codegen::gen_client \
    --with-watch \
    --with-applyconfig \
    --input-pkg-root totalsoft.ro/platform-controllers/pkg/apis \
    --output-pkg-root totalsoft.ro/platform-controllers/pkg/generated \
    --output-base "./../.." \
    --boilerplate "./hack/boilerplate.go.txt"

kube::codegen::gen_openapi \
    --input-pkg-root totalsoft.ro/platform-controllers/pkg/apis \
    --output-pkg-root totalsoft.ro/platform-controllers/pkg/generated \
    --output-base "./../.." \
    --boilerplate "./hack/boilerplate.go.txt"    