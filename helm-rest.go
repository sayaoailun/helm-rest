package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	restful "github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"helm.sh/helm/v3/cmd/helm/search"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type HelmResource struct {
	// typically reference a DAO (data-access-object)
}

var (
	settingsGlobal *cli.EnvSettings
	server         *http.Server
	container      *restful.Container
)

func init() {
	log.SetFlags(log.Llongfile)
	settingsGlobal = cli.New()

	var listenPort string
	pflag.CommandLine.StringVar(&listenPort, "port", "8080", "server listen port")
	pflag.CommandLine.StringVar(&settingsGlobal.KubeConfig, "kubeconfig", "config/kubeconfig", "path to the kubeconfig file")
	pflag.CommandLine.StringVar(&settingsGlobal.RepositoryConfig, "repository-config", ".helm/repository/repositories.yaml", "path to the file containing repository names and URLs")
	pflag.CommandLine.StringVar(&settingsGlobal.RepositoryCache, "repository-cache", ".helm/repository/cache", "path to the file containing cached repository indexes")
	pflag.Parse()

	container = restful.NewContainer()
	server = &http.Server{Addr: fmt.Sprintf(":%s", listenPort), Handler: container}

	HelmResource{}.Register()

	config := restfulspec.Config{
		WebServices:                   container.RegisteredWebServices(), // you control what services are visible
		APIPath:                       "/apidocs.json",
		PostBuildSwaggerObjectHandler: enrichSwaggerObject}
	container.Add(restfulspec.NewOpenAPIService(config))

	// Optionally, you can install the Swagger Service which provides a nice Web UI on your REST API
	// You need to download the Swagger HTML5 assets and change the FilePath location in the config below.
	// Open http://localhost:8080/apidocs/?url=http://localhost:8080/apidocs.json
	container.ServeMux.Handle("/apidocs/", http.StripPrefix("/apidocs/", http.FileServer(http.Dir(`swagger-ui`))))

	// Optionally, you may need to enable CORS for the UI to work.
	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		CookiesAllowed: false,
		Container:      restful.DefaultContainer}
	container.Filter(cors.Filter)
}

func (h HelmResource) listRepo(req *restful.Request, resp *restful.Response) {
	f, err := listRepo()
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	resp.WriteEntity(f)
}

func (h HelmResource) addRepo(req *restful.Request, resp *restful.Response) {
	repoinfo := repo.Entry{}
	req.ReadEntity(&repoinfo)
	err := addRepo(&repoinfo)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = fmt.Sprintf("%s has been added to your repositories", repoinfo.Name)
	resp.WriteHeaderAndEntity(http.StatusCreated, result)
}

func (h HelmResource) updateRepo(req *restful.Request, resp *restful.Response) {
	err := updateRepo()
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = `Update Complete. ⎈Happy Helming!⎈`
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) removeRepo(req *restful.Request, resp *restful.Response) {
	name := req.PathParameter("repo-name")
	names := []string{name}
	err := removeRepo(names)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = fmt.Sprintf("%s has been removed from your repositories", name)
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) searchRepo(req *restful.Request, resp *restful.Response) {
	keyword := req.QueryParameter("keyword")
	charts, err := searchRepo(keyword)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, charts)
}

func (h HelmResource) list(req *restful.Request, resp *restful.Response) {
	namespace := req.QueryParameter("namespace")
	releases, err := list(namespace)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, releases)
}

func (h HelmResource) getAll(req *restful.Request, resp *restful.Response) {
	releaseName := req.QueryParameter("release-name")
	namespace := req.QueryParameter("namespace")
	release, err := getAll(releaseName, namespace)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, release)
}

func (h HelmResource) install(req *restful.Request, resp *restful.Response) {
	releaseInfo := ReleaseInfo{}
	req.ReadEntity(&releaseInfo)
	jsonInfo, err := json.Marshal(releaseInfo)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	log.Println(string(jsonInfo))
	release, err := install(&releaseInfo)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, release)
}

func (h HelmResource) upgrade(req *restful.Request, resp *restful.Response) {
	releaseInfo := ReleaseInfo{}
	req.ReadEntity(&releaseInfo)
	jsonInfo, err := json.Marshal(releaseInfo)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	log.Println(string(jsonInfo))
	release, err := upgrade(&releaseInfo)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, release)
}

func (h HelmResource) uninstall(req *restful.Request, resp *restful.Response) {
	releases := strings.Split(req.QueryParameter("releases"), ",")
	namespace := req.QueryParameter("namespace")
	err := uninstall(releases, namespace)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = "uninstalled"
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) history(req *restful.Request, resp *restful.Response) {
	releaseName := req.QueryParameter("release-name")
	namespace := req.QueryParameter("namespace")
	max, errParse := strconv.Atoi(req.QueryParameter("max"))
	if errParse != nil {
		log.Println(errParse)
		result := &Result{}
		result.Result = false
		result.Error = errParse.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	history, err := history(releaseName, namespace, max)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, history)
}

func (h HelmResource) rollback(req *restful.Request, resp *restful.Response) {
	releaseInfo := ReleaseInfo{}
	req.ReadEntity(&releaseInfo)
	err := rollback(&releaseInfo)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = "Rollback was a success! Happy Helming!"
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) create(req *restful.Request, resp *restful.Response) {
	chartName := req.PathParameter("chart-name")
	err := create(chartName)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = "chart created successfully"
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) packageChart(req *restful.Request, resp *restful.Response) {
	chartName := req.PathParameter("chart-name")
	err := packageChart(chartName)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = "chart packaged successfully"
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) upload(req *restful.Request, resp *restful.Response) {
	repoName := req.PathParameter("repo-name")
	chartName := req.PathParameter("chart-name")
	chartPackage := req.PathParameter("chart-package-name")
	message, err := upload(chartPackage, chartName, repoName)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = message
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) getChartFile(req *restful.Request, resp *restful.Response) {
	chartName := req.PathParameter("chart-name")
	filePath := req.PathParameter("file-path")
	chartDir := ".helm/charts"
	// Ensure the chart directory exists
	err := os.MkdirAll(chartDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	file := fmt.Sprintf("%s/%s/%s", chartDir, chartName, filePath)
	http.ServeFile(
		resp.ResponseWriter,
		req.Request,
		file)
}

func (h HelmResource) editChartFile(req *restful.Request, resp *restful.Response) {
	chartName := req.PathParameter("chart-name")
	filePath := req.PathParameter("file-path")
	chartDir := ".helm/charts"
	// Ensure the chart directory exists
	err := os.MkdirAll(chartDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	file := fmt.Sprintf("%s/%s/%s", chartDir, chartName, filePath)
	dir, _ := filepath.Split(file)
	// Ensure the chart file path exists
	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	content, err := ioutil.ReadAll(req.Request.Body)
	if err != nil {
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	err = ioutil.WriteFile(file, content, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = "update successfully"
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) removeChartFile(req *restful.Request, resp *restful.Response) {
	chartName := req.PathParameter("chart-name")
	filePath := req.PathParameter("file-path")
	chartDir := ".helm/charts"
	// Ensure the chart directory exists
	err := os.MkdirAll(chartDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	file := fmt.Sprintf("%s/%s/%s", chartDir, chartName, filePath)
	err = os.Remove(file)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = "remove successfully"
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) getChartFiles(req *restful.Request, resp *restful.Response) {
	chartName := req.PathParameter("chart-name")
	chartDir := ".helm/charts"
	// Ensure the chart directory exists
	err := os.MkdirAll(chartDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	file := fmt.Sprintf("%s/%s", chartDir, chartName)
	files := []string{}
	err = filepath.Walk(file, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// filepath.SplitList(path)[2:]
		subPath, err := filepath.Rel(chartDir, path)
		if err != nil {
			return err
		}
		files = append(files, subPath)
		return nil
	})
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	resp.WriteHeaderAndEntity(http.StatusOK, files)
}

func (h HelmResource) chartList(req *restful.Request, resp *restful.Response) {
	chartDir := ".helm/charts"
	// Ensure the chart directory exists
	err := os.MkdirAll(chartDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	fileNames := []string{}
	files, err := os.ReadDir(chartDir)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}

	for _, file := range files {
		fileNames = append(fileNames, file.Name())
	}
	resp.WriteHeaderAndEntity(http.StatusOK, fileNames)
}

func (h HelmResource) removeChart(req *restful.Request, resp *restful.Response) {
	chartDir := ".helm/charts"
	// Ensure the chart directory exists
	err := os.MkdirAll(chartDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	chartName := req.PathParameter("chart-name")
	err = os.RemoveAll(fmt.Sprintf("%s/%s", chartDir, chartName))
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = "remove chart successfully"
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) packageList(req *restful.Request, resp *restful.Response) {
	chartPackgeDir := ".helm/chart-package"
	// Ensure the chart package directory exists
	err := os.MkdirAll(chartPackgeDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	chartName := req.PathParameter("chart-name")
	f, err := os.Open(fmt.Sprintf("%s/%s", chartPackgeDir, chartName))
	defer f.Close()
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	files, err := f.Readdir(-1)
	if err != nil {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	fileNames := []string{}
	for _, file := range files {
		fileNames = append(fileNames, file.Name())
	}
	resp.WriteHeaderAndEntity(http.StatusOK, fileNames)
}

func (h HelmResource) removePackage(req *restful.Request, resp *restful.Response) {
	chartPackgeDir := ".helm/chart-package"
	// Ensure the chart package directory exists
	err := os.MkdirAll(chartPackgeDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	chartName := req.PathParameter("chart-name")
	chartPackageName := req.PathParameter("chart-package-name")
	err = os.Remove(fmt.Sprintf("%s/%s/%s", chartPackgeDir, chartName, chartPackageName))
	if err != nil && !os.IsExist(err) {
		log.Println(err)
		result := &Result{}
		result.Result = false
		result.Error = err.Error()
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, result)
		return
	}
	result := &Result{}
	result.Result = true
	result.Message = "remove chart package successfully"
	resp.WriteHeaderAndEntity(http.StatusOK, result)
}

func (h HelmResource) Register() {
	charttags := []string{"chart"}
	releasetags := []string{"release"}
	repotags := []string{"repo"}

	ws := new(restful.WebService)
	ws.Path("/helm")
	ws.Consumes(restful.MIME_JSON)
	ws.Produces(restful.MIME_JSON)

	// repo
	ws.Route(ws.GET("/repo").To(h.listRepo).
		Doc("list chart repositories").
		Metadata(restfulspec.KeyOpenAPITags, repotags).
		Returns(http.StatusOK, "OK", repo.File{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.POST("/repo").To(h.addRepo).
		Doc("add chart repository").
		Metadata(restfulspec.KeyOpenAPITags, repotags).
		Reads(repo.Entry{}).
		Returns(http.StatusCreated, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.PUT("/repo").To(h.updateRepo).
		Doc("update chart repositories").
		Metadata(restfulspec.KeyOpenAPITags, repotags).
		Reads(EmptyBody{}).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.DELETE("/repo/{repo-name}").To(h.removeRepo).
		Doc("remove chart repository").
		Param(ws.PathParameter("repo-name", "name of the repo").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, repotags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))

	// search
	ws.Route(ws.GET("/search/repo").To(h.searchRepo).
		Doc("search chart in repository").
		Param(ws.QueryParameter("keyword", "keyword").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, repotags).
		Returns(http.StatusOK, "OK", []search.Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))

	// chart
	ws.Route(ws.POST("/chart/{chart-name}").To(h.create).
		Doc("create chart").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Reads(EmptyBody{}).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.GET("/chart").To(h.chartList).
		Doc("list chart").
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", []string{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.DELETE("/chart/{chart-name}").To(h.removeChart).
		Doc("remove chart").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.POST("/package/{chart-name}").To(h.packageChart).
		Doc("package chart").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Reads(EmptyBody{}).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.GET("/package/{chart-name}").To(h.packageList).
		Doc("list chart package").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", []string{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.DELETE("/package/{chart-name}/{chart-package-name}").To(h.removePackage).
		Doc("remove chart package").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Param(ws.PathParameter("chart-package-name", "name of chart package").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", []string{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.POST("/upload/{repo-name}/{chart-name}/{chart-package-name}").To(h.upload).
		Doc("upload chart").
		Param(ws.PathParameter("repo-name", "name of repo").DataType("string")).
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Param(ws.PathParameter("chart-package-name", "name of chart package").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Reads(EmptyBody{}).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.GET("/chart/{chart-name}/{file-path:*}").Produces("text/plain").To(h.getChartFile).
		Doc("get chart file").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Param(ws.PathParameter("file-path", "relative path of file").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "file content", "file content").
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.PUT("/chart/{chart-name}/{file-path:*}").Consumes("text/plain").To(h.editChartFile).
		Doc("edit chart file").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Param(ws.PathParameter("file-path", "relative path of file").DataType("string")).
		Reads(EmptyBody{}).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.DELETE("/chart/{chart-name}/{file-path:*}").To(h.removeChartFile).
		Doc("remove chart file").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Param(ws.PathParameter("file-path", "relative path of file").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.GET("/chart/{chart-name}").To(h.getChartFiles).
		Doc("get chart files").
		Param(ws.PathParameter("chart-name", "name of chart").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", []string{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))

	// release
	ws.Route(ws.GET("/list").To(h.list).
		Doc("list releases").
		Param(ws.QueryParameter("namespace", "namespace of the releases").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, releasetags).
		Returns(http.StatusOK, "OK", []release.Release{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.GET("/get/all").To(h.getAll).
		Doc("get release info").
		Param(ws.QueryParameter("release-name", "name of the release").DataType("string")).
		Param(ws.QueryParameter("namespace", "namespace of the release").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, releasetags).
		Returns(http.StatusOK, "OK", release.Release{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.POST("/install").To(h.install).
		Doc("install release").
		Reads(ReleaseInfo{}).
		Metadata(restfulspec.KeyOpenAPITags, releasetags).
		Returns(http.StatusOK, "OK", release.Release{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.GET("/history").To(h.history).
		Doc("get history of release").
		Param(ws.QueryParameter("release-name", "name of the release").DataType("string")).
		Param(ws.QueryParameter("namespace", "namespace of the release").DataType("string")).
		Param(ws.QueryParameter("max", "maximum number of revision to include in history").DataType("int")).
		Metadata(restfulspec.KeyOpenAPITags, releasetags).
		Returns(http.StatusOK, "OK", releaseHistory{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.PUT("/upgrade").To(h.upgrade).
		Doc("upgrade release").
		Reads(ReleaseInfo{}).
		Metadata(restfulspec.KeyOpenAPITags, releasetags).
		Returns(http.StatusOK, "OK", release.Release{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.PUT("/rollback").To(h.rollback).
		Doc("rollback release").
		Reads(ReleaseInfo{}).
		Metadata(restfulspec.KeyOpenAPITags, releasetags).
		Reads(EmptyBody{}).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.DELETE("/uninstall").To(h.uninstall).
		Doc("uninstall releases").
		Param(ws.QueryParameter("releases", "name of the releases(separated with commas)").DataType("string")).
		Param(ws.QueryParameter("namespace", "namespace of the releases").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, releasetags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))

	container.Add(ws)
}

func main() {
	go func() {
		log.Println(server.ListenAndServe())
	}()
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Println("http: Server shutting down...")
	err := server.Shutdown(ctx)
	if err != nil {
		log.Println(err)
	}
}

func enrichSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "Helm Rest Service",
			Description: "Rest service for helm",
			Contact: &spec.ContactInfo{
				ContactInfoProps: spec.ContactInfoProps{
					Name:  "Jianwei Guo",
					Email: "guojianwei007@126.com",
					URL:   "https://github.com/sayaoailun",
				},
			},
			License: &spec.License{
				LicenseProps: spec.LicenseProps{
					Name: "Apache License Version 2.0",
					URL:  "http://www.apache.org/licenses/LICENSE-2.0",
				},
			},
			Version: "1.0.0",
		},
	}
	swo.Tags = []spec.Tag{spec.Tag{TagProps: spec.TagProps{
		Name:        "chart",
		Description: "chart operation"}}, spec.Tag{TagProps: spec.TagProps{
		Name:        "release",
		Description: "release operation"}}, spec.Tag{TagProps: spec.TagProps{
		Name:        "repo",
		Description: "repo operation"}}}
}

// response result
type Result struct {
	Result  bool   `json:"result" description:"result" default:"false"`
	Message string `json:"message" description:"message" default:"string"`
	Error   string `json:"error" description:"error" default:"string"`
}

// information of release
type ReleaseInfo struct {
	Name      string   `json:"name" description:"name of release" default:"string"`
	Namespace string   `json:"namespace" description:"namespace of release" default:"string"`
	Chart     string   `json:"chart" description:"chart of release" default:"string"`
	Values    []string `json:"values" description:"values of release" default:"[]"`
	Version   int      `json:"version" description:"version of release" default:"0"`
}

// empty body
type EmptyBody struct {
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}

func debug(format string, v ...interface{}) {
	if settingsGlobal.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func warning(format string, v ...interface{}) {
	format = fmt.Sprintf("WARNING: %s\n", format)
	fmt.Fprintf(os.Stderr, format, v...)
}

func newSettings(namespace string) (*cli.EnvSettings, error) {
	s := cli.New()
	s.KubeConfig = settingsGlobal.KubeConfig
	s.RepositoryConfig = settingsGlobal.RepositoryConfig
	s.RepositoryCache = settingsGlobal.RepositoryCache
	config, ok := s.RESTClientGetter().(*genericclioptions.ConfigFlags)
	if ok {
		config.Namespace = &namespace
	} else {
		return nil, errors.New("namespace not set")
	}
	return s, nil
}

func newConfig(namespace string, settings *cli.EnvSettings) (*action.Configuration, error) {
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), debug); err != nil {
		log.Println(err)
		return nil, err
	}
	return cfg, nil
}
