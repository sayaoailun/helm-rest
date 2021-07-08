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
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
)

const uninstallDesc = `
This command takes a release name and uninstalls the release.

It removes all of the resources associated with the last release of the chart
as well as the release history, freeing it up for future use.

Use the '--dry-run' flag to see which releases will be uninstalled without actually
uninstalling them.
`

func uninstall(releaseNames []string, namespace string) error {
	out := os.Stdout

	s, err := newSettings(namespace)
	if err != nil {
		log.Println(err)
		return err
	}
	cfg, err := newConfig(namespace, s)
	if err != nil {
		log.Println(err)
		return err
	}

	client := action.NewUninstall(cfg)
	for i := 0; i < len(releaseNames); i++ {

		res, err := client.Run(releaseNames[i])
		if err != nil {
			log.Println(err)
			return err
		}
		if res != nil && res.Info != "" {
			fmt.Fprintln(out, res.Info)
		}

		fmt.Fprintf(out, "release \"%s\" uninstalled\n", releaseNames[i])
	}
	return nil
}
