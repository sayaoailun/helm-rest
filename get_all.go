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
	"log"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

var getAllHelp = `
This command prints a human readable collection of information about the
notes, hooks, supplied values, and generated manifest file of the given release.
`

func getAll(releaseName string, namespace string) (*release.Release, error) {
	s, err := newSettings(namespace)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	cfg, err := newConfig(namespace, s)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	client := action.NewGet(cfg)
	res, err := client.Run(releaseName)
	if err != nil {
		return nil, err
	}
	return res, nil
}
