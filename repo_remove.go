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
	"path/filepath"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

type repoRemoveOptions struct {
	names     []string
	repoFile  string
	repoCache string
}

func removeRepo(names []string) error {
	out := os.Stdout
	o := &repoRemoveOptions{}
	o.repoFile = settingsGlobal.RepositoryConfig
	o.repoCache = settingsGlobal.RepositoryCache
	o.names = names

	r, err := repo.LoadFile(o.repoFile)
	if isNotExist(err) || len(r.Repositories) == 0 {
		return errors.New("no repositories configured")
	}

	for _, name := range o.names {
		if !r.Remove(name) {
			return errors.Errorf("no repo named %q found", name)
		}
		if err := r.WriteFile(o.repoFile, 0644); err != nil {
			return err
		}

		if err := removeRepoCache(o.repoCache, name); err != nil {
			return err
		}
		fmt.Fprintf(out, "%q has been removed from your repositories\n", name)
	}

	return nil
}

func removeRepoCache(root, name string) error {
	idx := filepath.Join(root, helmpath.CacheChartsFile(name))
	if _, err := os.Stat(idx); err == nil {
		os.Remove(idx)
	}

	idx = filepath.Join(root, helmpath.CacheIndexFile(name))
	if _, err := os.Stat(idx); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "can't remove index file %s", idx)
	}
	return os.Remove(idx)
}
