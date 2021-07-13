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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
)

const packageDesc = `
This command packages a chart into a versioned chart archive file. If a path
is given, this will look at that path for a chart (which must contain a
Chart.yaml file) and then package that directory.

Versioned chart archives are used by Helm package repositories.

To sign a chart, use the '--sign' flag. In most cases, you should also
provide '--keyring path/to/secret/keys' and '--key keyname'.

  $ helm package --sign ./mychart --key mykey --keyring ~/.gnupg/secring.gpg

If '--keyring' is not specified, Helm usually defaults to the public keyring
unless your environment is otherwise configured.
`

func packageChart(chartName string) error {
	chartDir := ".helm/charts"
	// Ensure the chart directory exists
	err := os.MkdirAll(chartDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		return err
	}
	chartPackgeDir := ".helm/chart-package"
	// Ensure the chart package directory exists
	err = os.MkdirAll(chartPackgeDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		return err
	}
	chartPath := fmt.Sprintf("%s/%s", chartDir, chartName)
	client := action.NewPackage()
	client.Destination = fmt.Sprintf("%s/%s", chartPackgeDir, chartName)
	valueOpts := &values.Options{}
	client.RepositoryConfig = settingsGlobal.RepositoryConfig
	client.RepositoryCache = settingsGlobal.RepositoryCache
	p := getter.All(settingsGlobal)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(chartPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(chartPath); err != nil {
		return err
	}

	if client.DependencyUpdate {
		downloadManager := &downloader.Manager{
			Out:              ioutil.Discard,
			ChartPath:        path,
			Keyring:          client.Keyring,
			Getters:          p,
			Debug:            settingsGlobal.Debug,
			RepositoryConfig: settingsGlobal.RepositoryConfig,
			RepositoryCache:  settingsGlobal.RepositoryCache,
		}

		if err := downloadManager.Update(); err != nil {
			return err
		}
	}
	packageInfo, err := client.Run(path, vals)
	if err != nil {
		return err
	}
	out := os.Stdout
	fmt.Fprintf(out, "Successfully packaged chart and saved it to: %s\n", packageInfo)
	return nil
}
