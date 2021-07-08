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
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"

	"helm.sh/helm/v3/cmd/helm/search"
	"helm.sh/helm/v3/pkg/cli/output"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

const searchRepoDesc = `
Search reads through all of the repositories configured on the system, and
looks for matches. Search of these repositories uses the metadata stored on
the system.

It will display the latest stable versions of the charts found. If you
specify the --devel flag, the output will include pre-release versions.
If you want to search using a version constraint, use --version.

Examples:

    # Search for stable release versions matching the keyword "nginx"
    $ helm search repo nginx

    # Search for release versions matching the keyword "nginx", including pre-release versions
    $ helm search repo nginx --devel

    # Search for the latest stable release for nginx-ingress with a major version of 1
    $ helm search repo nginx-ingress --version ^1.0.0

Repositories are managed with 'helm repo' commands.
`

// searchMaxScore suggests that any score higher than this is not considered a match.
const searchMaxScore = 25

type searchRepoOptions struct {
	versions     bool
	regexp       bool
	devel        bool
	version      string
	maxColWidth  uint
	repoFile     string
	repoCacheDir string
	outputFormat output.Format
}

func searchRepo(keyword string) ([]*search.Result, error) {
	o := &searchRepoOptions{}
	o.repoFile = settingsGlobal.RepositoryConfig
	o.repoCacheDir = settingsGlobal.RepositoryCache
	o.setupSearchedVersion()

	index, err := o.buildIndex()
	if err != nil {
		return nil, err
	}

	var res []*search.Result
	if keyword == "" {
		res = index.All()
	} else {
		res, err = index.Search(keyword, searchMaxScore, o.regexp)
		if err != nil {
			return nil, err
		}
	}

	search.SortScore(res)
	data, err := o.applyConstraint(res)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (o *searchRepoOptions) setupSearchedVersion() {
	debug("Original chart version: %q", o.version)

	if o.version != "" {
		return
	}

	if o.devel { // search for releases and prereleases (alpha, beta, and release candidate releases).
		debug("setting version to >0.0.0-0")
		o.version = ">0.0.0-0"
	} else { // search only for stable releases, prerelease versions will be skip
		debug("setting version to >0.0.0")
		o.version = ">0.0.0"
	}
}

func (o *searchRepoOptions) applyConstraint(res []*search.Result) ([]*search.Result, error) {
	if o.version == "" {
		return res, nil
	}

	constraint, err := semver.NewConstraint(o.version)
	if err != nil {
		return res, errors.Wrap(err, "an invalid version/constraint format")
	}

	data := res[:0]
	foundNames := map[string]bool{}
	for _, r := range res {
		// if not returning all versions and already have found a result,
		// you're done!
		if !o.versions && foundNames[r.Name] {
			continue
		}
		v, err := semver.NewVersion(r.Chart.Version)
		if err != nil {
			continue
		}
		if constraint.Check(v) {
			data = append(data, r)
			foundNames[r.Name] = true
		}
	}

	return data, nil
}

func (o *searchRepoOptions) buildIndex() (*search.Index, error) {
	// Load the repositories.yaml
	rf, err := repo.LoadFile(o.repoFile)
	if isNotExist(err) || len(rf.Repositories) == 0 {
		return nil, errors.New("no repositories configured")
	}

	i := search.NewIndex()
	for _, re := range rf.Repositories {
		n := re.Name
		f := filepath.Join(o.repoCacheDir, helmpath.CacheIndexFile(n))
		ind, err := repo.LoadIndexFile(f)
		if err != nil {
			warning("Repo %q is corrupt or missing. Try 'helm repo update'.", n)
			warning("%s", err)
			continue
		}

		i.AddRepo(n, ind, o.versions || len(o.version) > 0)
	}
	return i, nil
}
