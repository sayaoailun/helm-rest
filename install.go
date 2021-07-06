/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"io"
	"log"
	"os"

	"github.com/pkg/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli/values"
	helmValues "helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
)

const installDesc = `
This command installs a chart archive.

The install argument must be a chart reference, a path to a packaged chart,
a path to an unpacked chart directory or a URL.

To override values in a chart, use either the '--values' flag and pass in a file
or use the '--set' flag and pass configuration from the command line, to force
a string value use '--set-string'. In case a value is large and therefore
you want not to use neither '--values' nor '--set', use '--set-file' to read the
single large value from file.

    $ helm install -f myvalues.yaml myredis ./redis

or

    $ helm install --set name=prod myredis ./redis

or

    $ helm install --set-string long_int=1234567890 myredis ./redis

or

    $ helm install --set-file my_script=dothings.sh myredis ./redis

You can specify the '--values'/'-f' flag multiple times. The priority will be given to the
last (right-most) file specified. For example, if both myvalues.yaml and override.yaml
contained a key called 'Test', the value set in override.yaml would take precedence:

    $ helm install -f myvalues.yaml -f override.yaml  myredis ./redis

You can specify the '--set' flag multiple times. The priority will be given to the
last (right-most) set specified. For example, if both 'bar' and 'newbar' values are
set for a key called 'foo', the 'newbar' value would take precedence:

    $ helm install --set foo=bar --set foo=newbar  myredis ./redis


To check the generated manifests of a release without installing the chart,
the '--debug' and '--dry-run' flags can be combined.

If --verify is set, the chart MUST have a provenance file, and the provenance
file MUST pass all verification steps.

There are five different ways you can express the chart you want to install:

1. By chart reference: helm install mymaria example/mariadb
2. By path to a packaged chart: helm install mynginx ./nginx-1.2.3.tgz
3. By path to an unpacked chart directory: helm install mynginx ./nginx
4. By absolute URL: helm install mynginx https://example.com/charts/nginx-1.2.3.tgz
5. By chart reference and repo url: helm install --repo https://example.com/charts/ mynginx nginx

CHART REFERENCES

A chart reference is a convenient way of referencing a chart in a chart repository.

When you use a chart reference with a repo prefix ('example/mariadb'), Helm will look in the local
configuration for a chart repository named 'example', and will then look for a
chart in that repository whose name is 'mariadb'. It will install the latest stable version of that chart
until you specify '--devel' flag to also include development version (alpha, beta, and release candidate releases), or
supply a version number with the '--version' flag.

To see the list of chart repositories, use 'helm repo list'. To search for
charts in a repository, use 'helm search'.
`

func install(releaseInfo *ReleaseInfo) (*release.Release, error) {
	cfg := actionConfig
	if err := cfg.Init(settings.RESTClientGetter(), releaseInfo.Namespace, os.Getenv("HELM_DRIVER"), debug); err != nil {
		log.Fatal(err)
		return nil, err
	}
	config, ok := settings.RESTClientGetter().(*genericclioptions.ConfigFlags)
	if ok {
		config.Namespace = &releaseInfo.Namespace
	} else {
		return nil, errors.New("namespace not set")
	}

	client := action.NewInstall(cfg)
	valueOpts := &helmValues.Options{}
	valueOpts.Values = releaseInfo.Values
	out := os.Stdout
	args := []string{releaseInfo.Name, releaseInfo.Chart}
	client.CreateNamespace = true
	rel, err := runInstall(args, client, valueOpts, out)
	if err != nil {
		return nil, err
	}
	return rel, nil
}

func runInstall(args []string, client *action.Install, valueOpts *values.Options, out io.Writer) (*release.Release, error) {
	debug("Original chart version: %q", client.Version)
	if client.Version == "" && client.Devel {
		debug("setting version to >0.0.0-0")
		client.Version = ">0.0.0-0"
	}

	name, chart, err := client.NameAndChart(args)
	if err != nil {
		return nil, err
	}
	client.ReleaseName = name

	cp, err := client.ChartPathOptions.LocateChart(chart, settings)
	if err != nil {
		return nil, err
	}

	debug("CHART PATH: %s\n", cp)

	p := getter.All(settings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return nil, err
	}

	if chartRequested.Metadata.Deprecated {
		warning("This chart is deprecated")
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              out,
					ChartPath:        cp,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return nil, errors.Wrap(err, "failed reloading chart after repo update")
				}
			} else {
				return nil, err
			}
		}
	}

	client.Namespace = settings.Namespace()
	return client.Run(chartRequested, vals)
}

// checkIfInstallable validates if a chart can be installed
//
// Application chart type is only installable
func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}
