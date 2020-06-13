package httpcache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

const (
	WorkerInpSize     = 20000
	DefaultNofWorkers = 3
)

type (
	ProxyCtxt struct {
		httpCacheCtxt *HttpCacheCtxt

		NoOfWorkers int
		Workers     []*ProxyWorker

		quitCh chan bool
	}

	ProxyWorker struct {
		Id    int
		InpCh chan *http.Request
		OutCh chan *http.Response

		proxyClient *http.Client
	}
)

func NewProxyCtxt(httpCacheCtxt *HttpCacheCtxt) (proxyCtxt *ProxyCtxt, err error) {

	proxyCtxt = &ProxyCtxt{

		httpCacheCtxt: httpCacheCtxt,
		NoOfWorkers:   runtime.NumCPU() * DefaultNofWorkers,

		quitCh: make(chan bool),
	}

	if proxyCtxt.httpCacheCtxt.Config.Proxy.NoOfWorkers != 0 {
		proxyCtxt.NoOfWorkers = proxyCtxt.httpCacheCtxt.Config.Proxy.NoOfWorkers
	}

	for idx := 0; idx < proxyCtxt.NoOfWorkers; idx++ {

		proxyCtxt.Workers = append(proxyCtxt.Workers, &ProxyWorker{
			Id:    idx,
			InpCh: make(chan *http.Request, WorkerInpSize),
			OutCh: make(chan *http.Response, WorkerInpSize),

			proxyClient: &http.Client{
				Transport: &http.Transport{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						return net.Dial("unix", proxyCtxt.httpCacheCtxt.Config.Server.RemoteHost)
					},
				},
			},
		})

	}

	return
}

func (proxyCtxt *ProxyCtxt) getNextWorkerIdx() (workerIdx int) {
	workerIdx = int(time.Now().Unix() % int64(proxyCtxt.NoOfWorkers))
	return
}

func (proxyCtxt *ProxyCtxt) Process() (err error) {

	for _, worker := range proxyCtxt.Workers {
		go worker.Process()
	}

	<-proxyCtxt.quitCh

	return
}

func (proxyCtxt *ProxyCtxt) Send(req *http.Request) (resp *http.Response, err error) {

	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered while sending request", r)
			err = errors.New("Failure to proxy request")
		}
	}()

	var (
		workerIdx int
		worker    *ProxyWorker

		reqKey ReqKeyT
	)

	reqKey = ReqKeyT(req.FormValue("uuid"))

	if reqKey == "" {
		err = errors.New("No CP found in the request body")
		return
	}

	workerIdx = proxyCtxt.getNextWorkerIdx()

	if worker = proxyCtxt.Workers[workerIdx]; worker == nil {
		err = errors.New("Worker not found with ID " + strconv.Itoa(workerIdx))
		return
	}

	if len(worker.InpCh) > (WorkerInpSize-1000) ||
		len(worker.OutCh) > (WorkerInpSize-1000) {

		err = errors.New("Worker busy, canceling request")
		return
	}

	worker.InpCh <- req

	resp = <-worker.OutCh

	return
}

func (proxyWorker *ProxyWorker) Process() (err error) {

	log.Println("Starting Worker with ID", proxyWorker.Id)

	for {
		select {
		case req := <-proxyWorker.InpCh:

			var (
				resp *http.Response
			)

			resp, err = proxyWorker.proxyRequest(req)

			proxyWorker.OutCh <- resp

		}
	}
	return
}

func (proxyWorker *ProxyWorker) proxyRequest(req *http.Request) (resp *http.Response, err error) {

	var (
		apiName string

		proxyReq *http.Request
	)

	apiName = req.URL.String()

	proxyReq, _ = http.NewRequest(req.Method,
		"http://unix"+apiName, req.Body)

	proxyReq.Header.Set("Host", req.Host)
	proxyReq.Header.Set("X-Forwarded-For", req.RemoteAddr)
	proxyReq.Header.Set("Authorization",
		fmt.Sprintf("X-LAVELLE-AUTH sessionid=%s", req.Header.Get("Authorization")))

	for header, values := range req.Header {
		for _, value := range values {
			proxyReq.Header.Add(header, value)
		}
	}

	if resp, err = proxyWorker.proxyClient.Do(proxyReq); err != nil {
		return
	}

	return
}
