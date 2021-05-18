module github.com/jpbetz/runner-webhook

go 1.16

require (
	github.com/containerd/containerd v1.5.1 // indirect
	github.com/docker/docker v20.10.6+incompatible
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/wasmerio/wasmer-go v1.0.3
	k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	sigs.k8s.io/structured-merge-diff/v4 v4.1.1
)
