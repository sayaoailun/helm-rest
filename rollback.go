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
	"os"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const rollbackDesc = `
This command rolls back a release to a previous revision.

The first argument of the rollback command is the name of a release, and the
second is a revision (version) number. If this argument is omitted, it will
roll back to the previous release.

To see revision numbers, run 'helm history RELEASE'.
`

func rollback(releaseInfo *ReleaseInfo) error {
	cfg := actionConfig
	if err := cfg.Init(settings.RESTClientGetter(), releaseInfo.Namespace, os.Getenv("HELM_DRIVER"), debug); err != nil {
		log.Fatal(err)
		return err
	}
	config, ok := settings.RESTClientGetter().(*genericclioptions.ConfigFlags)
	if ok {
		config.Namespace = &releaseInfo.Namespace
	} else {
		return errors.New("namespace not set")
	}

	client := action.NewRollback(cfg)
	client.Version = releaseInfo.Version

	if err := client.Run(releaseInfo.Name); err != nil {
		return err
	}

	return nil
}
