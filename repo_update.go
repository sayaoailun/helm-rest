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
	"os"
	"sync"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

const updateDesc = `
Update gets the latest information about charts from the respective chart repositories.
Information is cached locally, where it is used by commands like 'helm search'.
`

var errNoRepositories = errors.New("no repositories found. You must add one before updating")

type repoUpdateOptions struct {
	repoFile  string
	repoCache string
}

func updateRepo() error {
	out := os.Stdout
	o := &repoUpdateOptions{}
	o.repoFile = settingsGlobal.RepositoryConfig
	o.repoCache = settingsGlobal.RepositoryCache
	f, err := repo.LoadFile(o.repoFile)
	switch {
	case isNotExist(err):
		return errNoRepositories
	case err != nil:
		return errors.Wrapf(err, "failed loading file: %s", o.repoFile)
	case len(f.Repositories) == 0:
		return errNoRepositories
	}

	var repos []*repo.ChartRepository
	for _, cfg := range f.Repositories {
		r, err := repo.NewChartRepository(cfg, getter.All(settingsGlobal))
		if err != nil {
			return err
		}
		if o.repoCache != "" {
			r.CachePath = o.repoCache
		}
		repos = append(repos, r)
	}

	fmt.Fprintln(out, "Hang tight while we grab the latest from your chart repositories...")
	var wg sync.WaitGroup
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				fmt.Fprintf(out, "...Unable to get an update from the %q chart repository (%s):\n\t%s\n", re.Config.Name, re.Config.URL, err)
			} else {
				fmt.Fprintf(out, "...Successfully got an update from the %q chart repository\n", re.Config.Name)
			}
		}(re)
	}
	wg.Wait()
	fmt.Fprintln(out, "Update Complete. ???Happy Helming!???")
	return nil
}
