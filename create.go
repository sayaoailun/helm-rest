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
	"io"
	"log"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

const createDesc = `
This command creates a chart directory along with the common files and
directories used in a chart.

For example, 'helm create foo' will create a directory structure that looks
something like this:

    foo/
    ├── .helmignore   # Contains patterns to ignore when packaging Helm charts.
    ├── Chart.yaml    # Information about your chart
    ├── values.yaml   # The default values for your templates
    ├── charts/       # Charts that this chart depends on
    └── templates/    # The template files
        └── tests/    # The test files

'helm create' takes a path for an argument. If directories in the given path
do not exist, Helm will attempt to create them as it goes. If the given
destination exists and there are files in that directory, conflicting files
will be overwritten, but other files will be left alone.
`

func create(chartName string) error {
	chartDir := ".helm/charts"
	// Ensure the chart directory exists
	err := os.MkdirAll(chartDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		return err
	}
	o := &createOptions{}
	o.name = fmt.Sprintf("%s/%s", chartDir, chartName)
	o.starterDir = ".helm/starters"
	o.starter = ""
	out := os.Stdout
	return o.run(out)
}

type createOptions struct {
	starter    string // --starter
	name       string
	starterDir string
}

func (o *createOptions) run(out io.Writer) error {
	fmt.Fprintf(out, "Creating %s\n", o.name)

	chartname := filepath.Base(o.name)
	cfile := &chart.Metadata{
		Name:        chartname,
		Description: "A Helm chart for Kubernetes",
		Type:        "application",
		Version:     "0.1.0",
		AppVersion:  "0.1.0",
		APIVersion:  chart.APIVersionV2,
	}

	if o.starter != "" {
		// Create from the starter
		lstarter := filepath.Join(o.starterDir, o.starter)
		// If path is absolute, we don't want to prefix it with helm starters folder
		if filepath.IsAbs(o.starter) {
			lstarter = o.starter
		}
		return chartutil.CreateFrom(cfile, filepath.Dir(o.name), lstarter)
	}

	chartutil.Stderr = out
	_, err := chartutil.Create(chartname, filepath.Dir(o.name))
	return err
}
