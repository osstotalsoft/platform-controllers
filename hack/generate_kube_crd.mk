CODE_GENERATOR_DIR?=~/go/pkg/mod/k8s.io/code-generator@v0.28.2
# https://github.com/kubernetes/code-generator
#
# go install k8s.io/code-generator@latest
generate-apis:
	hack/update-codegen.sh


#https://book.kubebuilder.io/reference/controller-gen.html
# go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
generate-crd:
	controller-gen crd paths="totalsoft.ro/platform-controllers/pkg/apis/..." +output:dir=helm/crds