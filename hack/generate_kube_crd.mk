CODE_GENERATOR_DIR?=~/go/pkg/mod/k8s.io/code-generator@v0.23.5
# https://github.com/kubernetes/code-generator
#
# go install k8s.io/code-generator@vlatest
generate-apis:
	$(CODE_GENERATOR_DIR)/generate-groups.sh all \
       totalsoft.ro/platform-controllers/pkg/generated totalsoft.ro/platform-controllers/pkg/apis \
       "provisioning:v1alpha1 configuration:v1alpha1" \
       --output-base ./../.. \
       -h hack/boilerplate.go.txt


#https://book.kubebuilder.io/reference/controller-gen.html
# go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
generate-crd:
	controller-gen crd paths="totalsoft.ro/platform-controllers/pkg/apis/..." +output:dir=helm/crds