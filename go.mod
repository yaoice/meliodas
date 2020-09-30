module github.com/yaoice/meliodas

require (
	github.com/alexflint/go-filemutex v1.1.0
	github.com/containernetworking/cni v0.7.1
	github.com/containernetworking/plugins v0.7.5
	github.com/coreos/go-iptables v0.4.1
	github.com/evanphx/json-patch v4.2.0+incompatible // indirect
	github.com/godbus/dbus v0.0.0-20181101234600-2ff6f7ffd60f
	github.com/gophercloud/gophercloud v0.12.0
	github.com/imdario/mergo v0.3.10 // indirect
	github.com/j-keck/arping v1.0.1
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/sys v0.0.0-20191128015809-6d18c012aee9
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	k8s.io/api v0.19.0-rc.3
	k8s.io/apimachinery v0.19.0-rc.3
	k8s.io/client-go v0.19.0-rc.3
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.18.8
	k8s.io/utils v0.0.0-20200731180307-f00132d28269
)

replace (
	github.com/containernetworking/cni => github.com/containernetworking/cni v0.6.0
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/apiserver => k8s.io/apiserver v0.18.8
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.8
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.8
	k8s.io/code-generator => k8s.io/code-generator v0.18.8
	k8s.io/component-base => k8s.io/component-base v0.18.8
	k8s.io/cri-api => k8s.io/cri-api v0.18.8
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.8
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.8
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.8
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.8
	k8s.io/kubectl => k8s.io/kubectl v0.18.8
	k8s.io/kubelet => k8s.io/kubelet v0.18.8
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.8
	k8s.io/metrics => k8s.io/metrics v0.18.8
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.8
)

go 1.13
