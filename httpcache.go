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
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	BaseEtcDir           = "/etc/httpcache"
	HttpCacheApiConfFile = BaseEtcDir + "/apis.json"

	HttpServerType = "http"
	UnixServerType = "unix"

	DefaultServerHost = "127.0.0.1"
	DefaultServerPort = "8098"

	DefaultMonitorHost = "0.0.0.0"
	DefaultMonitorPort = "9091"

	DefaultLogFile = "/tmp/httpcache.log"
)

type (
	ReqKeyT string

	FuncHandler func(http.ResponseWriter, *http.Request) ([]byte, error)

	Config struct {
		Server struct {
			ServerType string `json:"server_type"`
			LocalHost  string `json:"local_host"`
			RemoteHost string `json:"remote_host"`
			Port       string `json:"port"`
		} `json:"server"`

		Monitor struct {
			Host               string `json:"host"`
			Port               string `json:"port"`
			ShouldCollectStats bool   `json:"should_collect_stats"`
		} `json:"monitor"`

		Proxy struct {
			NoOfWorkers int `json:"no_of_workers"`
		} `json:"proxy"`

		Logger struct {
			LogFile string `json:"log_file"`
		} `json:"logger"`

		SkipCacheApis []string `json:"skip_cache_apis"`
	}

	HttpCacheCtxt struct {
		Cache     *Cache
		ProxyCtxt *ProxyCtxt

		Stats *Stats

		Config *Config

		logger *logrus.Logger

		Server           *http.Server
		MonitoringServer *http.Server

		SkipCacheMap       map[string]bool
		LocalCacheBuildMap map[string]FuncHandler
		Middlewares        []func(http.Handler) http.Handler
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

	if cfg.Monitor.Host == "" {
		cfg.Monitor.Host = DefaultMonitorHost
	}

	if cfg.Monitor.Port == "" {
		cfg.Monitor.Port = DefaultMonitorPort
	}

	if cfg.Server.Port == "" {
		cfg.Server.Port = DefaultServerPort
	}

	if cfg.Logger.LogFile == "" {
		cfg.Logger.LogFile = DefaultLogFile
	}

	log.Println(cfg)

	return

}
func NewHttpCacheCtxt() (httpCacheCtxt *HttpCacheCtxt, err error) {

	httpCacheCtxt = &HttpCacheCtxt{

		SkipCacheMap:       make(map[string]bool),
		LocalCacheBuildMap: make(map[string]FuncHandler),
	}

	if httpCacheCtxt.Config, err = NewHttpCacheConfig(); err != nil {
		return
	}

	if httpCacheCtxt.logger, err = NewLogger(httpCacheCtxt); err != nil {
		return
	}

	if httpCacheCtxt.ProxyCtxt, err = NewProxyCtxt(httpCacheCtxt); err != nil {
		return
	}

	if httpCacheCtxt.Stats, err = NewStats(); err != nil {
		return
	}

	if httpCacheCtxt.MonitoringServer, err = NewMonitorServer(); err != nil {
		return
	}

	if httpCacheCtxt.Cache, err = NewCache(httpCacheCtxt); err != nil {
		return
	}

	if httpCacheCtxt.Server, err = httpCacheCtxt.registerRoutes(); err != nil {
		return
	}

	if err = httpCacheCtxt.prepareSkipCacheMap(); err != nil {
		return
	}

	return
}

func NewLogger(httpCacheCtxt *HttpCacheCtxt) (logger *logrus.Logger, err error) {

	var (
		logFile *os.File
	)

	if logFile, err = os.OpenFile(httpCacheCtxt.Config.Logger.LogFile,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err != nil {

		return
	}

	logger = logrus.New()

	logger.SetOutput(logFile)

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

func (httpCacheCtxt *HttpCacheCtxt) RegisterLocalCacheHandler(api string,
	handler func(http.ResponseWriter,
		*http.Request) ([]byte, error)) (err error) {

	httpCacheCtxt.LocalCacheBuildMap[api] = handler

	return
}

func (httpCacheCtxt *HttpCacheCtxt) RegisterMiddleware(middleware func(http.Handler) http.Handler) (err error) {

	httpCacheCtxt.Middlewares = append(httpCacheCtxt.Middlewares, middleware)

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

	reqKey = ReqKeyT(req.FormValue("uuid"))

	if reqKey == "" {
		err = errors.New("No CP found in the request body")
		return
	}

	apiName = req.URL.Path

	httpCacheCtxt.logger.WithFields(logrus.Fields{
		"req_key":    reqKey,
		"api_name":   apiName,
		"event_type": "cache_requests",
	}).Info("Cache Request received")

	// Check if the request is part of the SkipCacheMap
	_, isPresent = httpCacheCtxt.SkipCacheMap[apiName]

	// Check if the cache is valid
	if isPresent != true {

		isCacheValid = httpCacheCtxt.Cache.IsValid(reqKey, apiName)

	} else {

		httpCacheCtxt.logger.WithFields(logrus.Fields{
			"req_key":    reqKey,
			"api_name":   apiName,
			"event_type": "cache_skipped",
		}).Info("Cache Request Skipped")

		httpCacheCtxt.Stats.Counter.Skipped.Inc()
		isCacheValid = false
	}

	if isCacheValid {

		httpCacheCtxt.logger.WithFields(logrus.Fields{
			"req_key":    reqKey,
			"api_name":   apiName,
			"event_type": "cache_response",
		}).Info("Cache Request Valid")

		httpCacheCtxt.Stats.Counter.CachedResponse.Inc()
		respBody, err = httpCacheCtxt.Cache.GetData(reqKey, apiName)
		return
	}

	// Check if the cache is to built by the local process
	// instead of proxying
	if handler, isPresent = httpCacheCtxt.LocalCacheBuildMap[apiName]; isPresent {

		httpCacheCtxt.logger.WithFields(logrus.Fields{
			"req_key":    reqKey,
			"api_name":   apiName,
			"event_type": "cache_local_handled",
		}).Info("Cache Request Local Handling")

		httpCacheCtxt.Stats.Counter.LocalHandled.Inc()

		if respBody, err = handler(w, req); err == nil {
			return
		}
		return
	}

	httpCacheCtxt.logger.WithFields(logrus.Fields{
		"req_key":    reqKey,
		"api_name":   apiName,
		"event_type": "cache_proxied",
	}).Info("Cache Request Proxied")

	httpCacheCtxt.Stats.Counter.Proxied.Inc()

	if resp, err = httpCacheCtxt.ProxyCtxt.Send(req); err != nil {
		if resp != nil {
			resp.Body.Close()
		}

		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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

	httpCacheCtxt.logger.WithFields(logrus.Fields{
		"req_key":    reqKey,
		"api_name":   apiName,
		"event_type": "cache_added",
	}).Info("Cache Response added")

	httpCacheCtxt.Stats.Counter.CacheAdded.Inc()

	httpCacheCtxt.Cache.Add(reqKey, apiName, respBody)

	return
}

func (httpCacheCtxt *HttpCacheCtxt) invalidateCacheHandler(w http.ResponseWriter, req *http.Request) {

	var (
		reqKey ReqKeyT
		err    error
	)

	reqKey = ReqKeyT(req.FormValue("uuid"))

	httpCacheCtxt.Stats.Counter.Invalidations.Inc()

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

	httpCacheCtxt.Stats.Counter.Requests.Inc()

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

func (httpCacheCtxt *HttpCacheCtxt) registerRoutes() (server *http.Server, err error) {

	var (
		router *mux.Router
	)

	router = mux.NewRouter()
	router.HandleFunc("/httpCache/invalidate", httpCacheCtxt.invalidateCacheHandler)

	router.PathPrefix("/").HandlerFunc(httpCacheCtxt.rootHandler)

	router.Use(httpCacheCtxt.loggingMiddleware)

	for _, middleware := range httpCacheCtxt.Middlewares {
		router.Use(middleware)
	}

	server = &http.Server{
		Handler:      router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return
}

func (httpCacheCtxt *HttpCacheCtxt) startMonitoringServer() (err error) {

	var (
		sockListener net.Listener
	)

	if sockListener, err = net.Listen("tcp4",
		fmt.Sprintf("%s:%s",
			httpCacheCtxt.Config.Monitor.Host,
			httpCacheCtxt.Config.Monitor.Port)); err != nil {

		panic(err)
	}

	log.Fatal(httpCacheCtxt.MonitoringServer.Serve(sockListener))

	return
}

func (httpCacheCtxt *HttpCacheCtxt) startListening() (err error) {

	var (
		sockListener net.Listener
	)

	if httpCacheCtxt.Config.Server.ServerType == UnixServerType {

		os.Remove(httpCacheCtxt.Config.Server.LocalHost)

		sockListener, err = net.Listen("unix",
			httpCacheCtxt.Config.Server.LocalHost)

	} else {

		sockListener, err = net.Listen("tcp4",
			fmt.Sprintf("%s:%s",
				httpCacheCtxt.Config.Server.LocalHost,
				httpCacheCtxt.Config.Server.Port))

	}

	if err != nil {
		panic(err)
	}

	log.Fatal(httpCacheCtxt.Server.Serve(sockListener))

	return
}

func (httpCacheCtxt *HttpCacheCtxt) Process() (err error) {

	go httpCacheCtxt.ProxyCtxt.Process()
	go httpCacheCtxt.startMonitoringServer()

	httpCacheCtxt.startListening()

	return
}
