package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"flag"

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
	"k8s.io/klog/v2"
)

type HelmResource struct {
	// typically reference a DAO (data-access-object)
}

var settings = cli.New()
var actionConfig = new(action.Configuration)

func init() {
	settings.KubeConfig = `C:\Users\guoji\Desktop\config`
	settings.RepositoryConfig = `C:\Users\guoji\Documents\.helm\repository\repositories.yaml`
	settings.RepositoryCache = `C:\Users\guoji\Documents\.helm\repository\cache`
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

func (h HelmResource) Register() {
	charttags := []string{"chart"}
	releasetags := []string{"release"}

	ws := new(restful.WebService)
	ws.Path("/helm")
	ws.Consumes(restful.MIME_JSON)
	ws.Produces(restful.MIME_JSON)

	// repo
	ws.Route(ws.GET("/repo").To(h.listRepo).
		Doc("list chart repositories").
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", repo.File{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.POST("/repo").To(h.addRepo).
		Doc("add chart repository").
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Reads(repo.Entry{}).
		Returns(http.StatusCreated, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.PUT("/repo").To(h.updateRepo).
		Doc("update chart repositories").
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.DELETE("/repo/{repo-name}").To(h.removeRepo).
		Doc("remove chart repository").
		Param(ws.PathParameter("repo-name", "name of the repo").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))

	// search
	ws.Route(ws.GET("/search/repo").To(h.searchRepo).
		Doc("search chart in repository").
		Param(ws.QueryParameter("keyword", "keyword").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, charttags).
		Returns(http.StatusOK, "OK", []search.Result{}).
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
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))
	ws.Route(ws.DELETE("/uninstall").To(h.uninstall).
		Doc("uninstall releases").
		Param(ws.QueryParameter("releases", "name of the releases(separated with commas)").DataType("string")).
		Param(ws.QueryParameter("namespace", "namespace of the releases").DataType("string")).
		Metadata(restfulspec.KeyOpenAPITags, releasetags).
		Returns(http.StatusOK, "OK", Result{}).
		Returns(http.StatusInternalServerError, "inner error", Result{}))

	restful.Add(ws)
}

func main() {
	HelmResource{}.Register()

	config := restfulspec.Config{
		WebServices:                   restful.RegisteredWebServices(), // you control what services are visible
		APIPath:                       "/apidocs.json",
		PostBuildSwaggerObjectHandler: enrichSwaggerObject}
	restful.DefaultContainer.Add(restfulspec.NewOpenAPIService(config))

	// Optionally, you can install the Swagger Service which provides a nice Web UI on your REST API
	// You need to download the Swagger HTML5 assets and change the FilePath location in the config below.
	// Open http://localhost:8080/apidocs/?url=http://localhost:8080/apidocs.json
	http.Handle("/apidocs/", http.StripPrefix("/apidocs/", http.FileServer(http.Dir(`C:\Users\guoji\Documents\git\github\swagger-api\swagger-ui\dist`))))

	// Optionally, you may need to enable CORS for the UI to work.
	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		CookiesAllowed: false,
		Container:      restful.DefaultContainer}
	restful.DefaultContainer.Filter(cors.Filter)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func enrichSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "Helm Rest Service",
			Description: "Rest service for helm",
			Contact: &spec.ContactInfo{
				ContactInfoProps: spec.ContactInfoProps{
					Name:  "郭建伟",
					Email: "guojwe@dcits.com",
					URL:   "http://dcits.com/",
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
		Description: "release operation"}}}
}

// 封装返回结果
type Result struct {
	Result  bool   `json:"result" description:"请求返回结果" default:"false"`
	Message string `json:"message" description:"请求返回信息描述" default:"string"`
	Error   string `json:"error" description:"请求错误信息" default:"string"`
}

// 创建或者更新release使用的info
type ReleaseInfo struct {
	Name      string   `json:"name" description:"release的名称" default:"string"`
	Namespace string   `json:"namespace" description:"release的命名空间" default:"string"`
	Chart     string   `json:"chart" description:"release使用的chart" default:"string"`
	Values    []string `json:"values" description:"release使用的values" default:"[]"`
	Version   int      `json:"version" description:"release的版本号" default:"0"`
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}

func debug(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func warning(format string, v ...interface{}) {
	format = fmt.Sprintf("WARNING: %s\n", format)
	fmt.Fprintf(os.Stderr, format, v...)
}

// addKlogFlags adds flags from k8s.io/klog
// marks the flags as hidden to avoid polluting the help text
func addKlogFlags(fs *pflag.FlagSet) {
	local := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(local)
	local.VisitAll(func(fl *flag.Flag) {
		fl.Name = normalize(fl.Name)
		if fs.Lookup(fl.Name) != nil {
			return
		}
		newflag := pflag.PFlagFromGoFlag(fl)
		newflag.Hidden = true
		fs.AddFlag(newflag)
	})
}

// normalize replaces underscores with hyphens
func normalize(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}
