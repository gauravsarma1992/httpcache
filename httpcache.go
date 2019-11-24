package httpcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

const (
	BaseEtcDir           = "/etc/httpcache"
	HttpCacheApiConfFile = BaseEtcDir + "/apis.json"

	HttpServerType = "http"
	UnixServerType = "unix"
)

type (
	ReqKeyT string

	FuncHandler func(http.ResponseWriter, *http.Request) ([]byte, error)

	Config struct {
		Server struct {
			Diagnose   bool   `json:"diagnose"`
			ServerType string `json:"server_type"`
			LocalHost  string `json:"local_host"`
			RemoteHost string `json:"remote_host"`
			DiagHost   string `json:"diag_host"`
			Port       string `json:"port"`
		} `json:"server"`

		Proxy struct {
			NoOfWorkers int `json:"no_of_workers"`
		} `json:"proxy"`

		SkipCacheApis []string `json:"skip_cache_apis"`
	}

	HttpCacheCtxt struct {
		Cache       *Cache
		ProxyCtxt   *ProxyCtxt
		CommandCtxt *CommandCtxt

		Config *Config

		SkipCacheMap       map[string]bool
		LocalCacheBuildMap map[string]FuncHandler

		SysStats   *SysStats
		CpStats    *sync.Map
		CpApiStats *sync.Map
		ProxyStats *sync.Map
	}
)

var (
	CommonErrMsg = []byte("{\"status\":\"failure\"}")
	AuthErrorMsg = []byte("{\"status\":\"unauthorized\"}")

	ApiLevels = []string{"/", "/api/v1/",
		"/api/v2/", "/api/v3/", "/api/v4/"}
)

func NewHttpCacheConfig() (cfg *Config, err error) {

	var (
		cfgFile []byte
	)

	cfg = &Config{}

	if cfgFile, err = ioutil.ReadFile(HttpCacheApiConfFile); err != nil {
		return
	}

	if err = json.Unmarshal(cfgFile, cfg); err != nil {
		return
	}

	return

}
func NewHttpCacheCtxt() (httpCacheCtxt *HttpCacheCtxt, err error) {

	httpCacheCtxt = &HttpCacheCtxt{

		SysStats: &SysStats{},

		CpStats:    &sync.Map{},
		CpApiStats: &sync.Map{},
		ProxyStats: &sync.Map{},

		SkipCacheMap:       make(map[string]bool),
		LocalCacheBuildMap: make(map[string]FuncHandler),
	}

	if httpCacheCtxt.Config, err = NewHttpCacheConfig(); err != nil {
		return
	}

	if httpCacheCtxt.ProxyCtxt, err = NewProxyCtxt(httpCacheCtxt); err != nil {
		return
	}

	if httpCacheCtxt.CommandCtxt, err = NewCommandCtxt(httpCacheCtxt); err != nil {
		return
	}

	if httpCacheCtxt.Cache, err = NewCache(httpCacheCtxt); err != nil {
		return
	}

	if err = httpCacheCtxt.prepareSkipCacheMap(); err != nil {
		return
	}

	if err = httpCacheCtxt.prepareLocalCacheBuildMap(); err != nil {
		return
	}

	return
}

func (httpCacheCtxt *HttpCacheCtxt) prepareSkipCacheMap() (err error) {

	for _, apiLevel := range ApiLevels {

		for _, api := range httpCacheCtxt.Config.SkipCacheApis {
			httpCacheCtxt.SkipCacheMap[fmt.Sprintf("%s%s",
				apiLevel, api)] = true
		}

	}

	return
}

func (httpCacheCtxt *HttpCacheCtxt) prepareLocalCacheBuildMap() (err error) {

	//httpCacheCtxt.LocalCacheBuildMap["/api/v2/login/"] = httpCacheCtxt.loginHandler
	return
}

func (httpCacheCtxt *HttpCacheCtxt) processRequest(w http.ResponseWriter,
	req *http.Request) (respBody []byte, err error) {

	var (
		isPresent    bool
		isCacheValid bool
		handler      FuncHandler
		resp         *http.Response

		reqKey  ReqKeyT
		apiName string
	)

	httpCacheCtxt.regSysStats(RpsStatKey)

	reqKey = ReqKeyT(req.FormValue("uuid"))

	if reqKey == "" {
		err = errors.New("No CP found in the request body")
		return
	}

	apiName = req.URL.Path

	// Check if the request is part of the SkipCacheMap
	_, isPresent = httpCacheCtxt.SkipCacheMap[apiName]

	// Check if the cache is valid
	if isPresent != true {

		isCacheValid = httpCacheCtxt.Cache.IsValid(reqKey, apiName)

		if !isCacheValid {
			httpCacheCtxt.regCpApiStats(reqKey, apiName, CacheMissStatKey)
		}

	} else {
		isCacheValid = false
	}

	if isCacheValid {
		httpCacheCtxt.regCpApiStats(reqKey, apiName, CacheHitStatKey)
		respBody, err = httpCacheCtxt.Cache.GetData(reqKey, apiName)
		return
	}

	// Check if the cache is to built by the local process
	// instead of proxying
	if handler, isPresent = httpCacheCtxt.LocalCacheBuildMap[apiName]; isPresent {
		httpCacheCtxt.regCpApiStats(reqKey, apiName, LocalCacheStatKey)
		if respBody, err = handler(w, req); err == nil {
			return
		}
		return
	}

	if resp, err = httpCacheCtxt.ProxyCtxt.Send(req); err != nil {
		if resp != nil {
			resp.Body.Close()
		}

		httpCacheCtxt.regCpApiStats(reqKey, apiName, BackendFailStatKey)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		httpCacheCtxt.regCpApiStats(reqKey, apiName, BackendFailStatKey)
		return
	}

	if respBody, err = ioutil.ReadAll(resp.Body); err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return
	}

	// This is used to build the cache with
	// the response received from the proxying
	// of the request to httpCache. The cache isn't
	// built when it is handled in the proxy service
	// locally. In case it is required to be handled
	// locally as well, custom logic has to be
	// written for it
	httpCacheCtxt.Cache.Add(reqKey, apiName, respBody)

	return
}

func (httpCacheCtxt *HttpCacheCtxt) invalidateCacheHandler(w http.ResponseWriter, req *http.Request) {

	var (
		reqKey ReqKeyT
		err    error
	)

	reqKey = ReqKeyT(req.FormValue("uuid"))

	httpCacheCtxt.regCpStats(reqKey, CacheInvalidationsStatKey)

	if err = httpCacheCtxt.Cache.Invalidate(reqKey); err != nil {

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(CommonErrMsg)

		return
	}

	fmt.Fprint(w, [1]byte{1})

	return
}

func (httpCacheCtxt *HttpCacheCtxt) rootHandler(w http.ResponseWriter, req *http.Request) {

	var (
		respBody []byte
		err      error
	)

	if respBody, err = httpCacheCtxt.processRequest(w, req); err != nil {

		if ProxyPresentErrorMessage == err.Error() {
			return
		}

		log.Println(err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(CommonErrMsg)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)

	return
}

func (httpCacheCtxt *HttpCacheCtxt) getActiveItems() (reqKeys []ReqKeyT, err error) {

	httpCacheCtxt.CpApiStats.Range(func(key interface{}, _ interface{}) bool {
		reqKeys = append(reqKeys, key.(ReqKeyT))
		return true
	})

	return
}

func (httpCacheCtxt *HttpCacheCtxt) getApisOfItem(reqKey ReqKeyT) (apis []string, err error) {

	httpCacheCtxt.CpApiStats.Range(func(key interface{}, val interface{}) bool {

		if key.(ReqKeyT) != reqKey && reqKey != "*" {
			return true
		}

		apiMap := val.(*sync.Map)

		apiMap.Range(func(k2 interface{}, _ interface{}) bool {
			apis = append(apis, k2.(string))
			return true
		})

		return true
	})

	return
}

func (httpCacheCtxt *HttpCacheCtxt) startListening() (err error) {

	var (
		router       *mux.Router
		server       *http.Server
		sockListener net.Listener
	)

	router = mux.NewRouter()
	router.HandleFunc("/httpCache/invalidate", httpCacheCtxt.invalidateCacheHandler)

	router.PathPrefix("/").HandlerFunc(httpCacheCtxt.rootHandler)

	router.Use(httpCacheCtxt.loggingMiddleware)

	server = &http.Server{
		Handler:      router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	if httpCacheCtxt.Config.Server.ServerType == UnixServerType {

		os.Remove(httpCacheCtxt.Config.Server.LocalHost)

		sockListener, err := net.Listen("unix",
			httpCacheCtxt.Config.Server.LocalHost)

		if err != nil {
			panic(err)
		}

		log.Fatal(server.Serve(sockListener))

	} else {

		sockListener, err = net.Listen("tcp4", ":8098")

		log.Fatal(server.Serve(sockListener))

	}

	return
}

func (httpCacheCtxt *HttpCacheCtxt) monitor() (err error) {

	for {

		time.Sleep(time.Second * 10)

		if httpCacheCtxt.Config.Server.Diagnose {

			var (
				totalItems int
			)

			httpCacheCtxt.CpApiStats.Range(func(_, _ interface{}) bool {
				totalItems++

				return true
			})

			log.Printf("Items - %d | Goroutines - %d | RPS - %d ",
				totalItems, runtime.NumGoroutine(), httpCacheCtxt.SysStats.Rps)
		}

	}
}

func (httpCacheCtxt *HttpCacheCtxt) Process() (err error) {

	go httpCacheCtxt.CommandCtxt.Process()
	go httpCacheCtxt.ProxyCtxt.Process()

	go httpCacheCtxt.monitor()

	httpCacheCtxt.startListening()

	return
}
