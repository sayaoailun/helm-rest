package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	restful "github.com/emicklei/go-restful/v3"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/repo"
)

func upload(chartPackage string, chartName string, repoName string) (string, error) {
	repos, err := repo.LoadFile(settingsGlobal.RepositoryConfig)
	if err != nil {
		return "", err
	}
	repoEntry := repos.Get(repoName)
	if repoEntry != nil {
		chartPackgeDir := ".helm/chart-package"
		// Ensure the chart package directory exists
		err := os.MkdirAll(chartPackgeDir, os.ModePerm)
		if err != nil && !os.IsExist(err) {
			log.Println(err)
			return "", err
		}
		chartPath := fmt.Sprintf("%s/%s/%s", chartPackgeDir, chartName, chartPackage)
		file, err := os.Open(chartPath)
		if err != nil {
			return "", err
		}
		defer file.Close()
		uploadUrl := fmt.Sprintf("%s/api/charts", repoEntry.URL)
		resp, err := http.Post(uploadUrl, restful.MIME_OCTET, file)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		message, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(message), nil
	} else {
		return "", errors.New("repo dose not exist")
	}
}
