# https://github.com/pulumi/crd2pulumi
#
# go install github.com/pulumi/crd2pulumi@latest
generate-pulumi-fluxcd-apis:
	crd2pulumi --goPath ./internal/controllers/provisioning/provisioners/pulumi/fluxcd/generated ./internal/controllers/provisioning/provisioners/pulumi/fluxcd/helm.toolkit.fluxcd.io_helmreleases.yaml


