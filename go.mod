module github.com/open-cluster-management/multicloud-operators-subscription-release

go 1.13

require (
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-openapi/spec v0.19.4
	github.com/onsi/gomega v1.9.0
	github.com/operator-framework/operator-sdk v0.18.0 // version upgrade should check for helmrelease_types.go changes
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.0
	golang.org/x/net v0.0.0-20200301022130-244492dfa37a
	gopkg.in/src-d/go-git.v4 v4.13.1
	helm.sh/helm/v3 v3.2.0
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200121204235-bf4fb3bd569c
	sigs.k8s.io/controller-runtime v0.6.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.2
)
