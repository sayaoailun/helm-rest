module github.com/sayaoailun/helm-rest

go 1.16

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/emicklei/go-restful-openapi/v2 v2.3.0
	github.com/emicklei/go-restful/v3 v3.5.1
	github.com/go-openapi/spec v0.20.3
	github.com/gofrs/flock v0.8.0
	github.com/gosuri/uitable v0.0.4
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d
	golang.org/x/tools v0.1.4 // indirect
	helm.sh/helm/v3 v3.6.1
	k8s.io/cli-runtime v0.21.0
	k8s.io/klog/v2 v2.8.0
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.8
