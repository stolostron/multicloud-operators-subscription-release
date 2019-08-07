module github.ibm.com/IBMMulticloudPlatform/subscription-operator

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/cameront/go-jsonpatch v0.0.0-20180223123257-a8710867776e
	github.com/ghodss/yaml v1.0.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a
	github.com/operator-framework/operator-sdk v0.9.1-0.20190724001845-d6e1aba9fa51
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.3.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.ibm.com/IBMMulticloudPlatform/placementrule v0.0.0-20190730162031-372256eedb81
	github.ibm.com/IBMMulticloudPlatform/subscription v0.0.0-20190802063328-ac8eeb15abc8
	github.ibm.com/dominique-vernier/helm-operator-test v0.0.0-20190802151104-52d3fe62e417
	golang.org/x/build v0.0.0-20190314133821-5284462c4bec
	golang.org/x/tools v0.0.0-20190710153321-831012c29e42 // indirect
	k8s.io/api v0.0.0-20190612125737-db0771252981
	k8s.io/apimachinery v0.0.0-20190612125636-6a5db36e93ad
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/helm v2.13.1+incompatible
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208 // indirect
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools v0.1.10
)

// Pinned to kubernetes-1.13.4
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190228174905-79427f02047f
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190228180923-a9e421a79326
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20181117043124-c2090bec4d9b
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190228175259-3e0149950b0e
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20180711000925-0cf8f7e6ed1d
	k8s.io/kubernetes => k8s.io/kubernetes v1.13.4
)

replace (
	github.com/coreos/prometheus-operator => github.com/coreos/prometheus-operator v0.29.0
	k8s.io/kube-state-metrics => k8s.io/kube-state-metrics v1.6.0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.1.11-0.20190411181648-9d55346c2bde
)
