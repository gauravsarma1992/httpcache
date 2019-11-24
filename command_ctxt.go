package httpcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	TurnOnCommand         = "turn-on-diag"
	TurnOffCommand        = "turn-off-diag"
	DumpStatsCommand      = "dump-stats"
	ShowSummaryCommand    = "show-summary"
	ShowProxyStatsCommand = "show-proxy-stats"
	ShowCacheStatsCommand = "show-cache-stats"
	ShowBEStatsCommand    = "show-backend-stats"
	HelpCommand           = "help"

	DumpFile = "/tmp/dump-stats-httpCache.json"

	LineDelimiter = "========================================="
)

type (
	CommandCtxt struct {
		httpCacheCtxt *HttpCacheCtxt
		listener      net.Listener

		noOfConns uint32
	}
)

func NewCommandCtxt(httpCacheCtxt *HttpCacheCtxt) (cmdCtxt *CommandCtxt, err error) {

	cmdCtxt = &CommandCtxt{
		httpCacheCtxt: httpCacheCtxt,
	}
	return
}

func (cmdCtxt *CommandCtxt) startListening() (err error) {

	if err = os.RemoveAll(cmdCtxt.httpCacheCtxt.Config.Server.DiagHost); err != nil {
		return
	}

	if cmdCtxt.listener, err = net.Listen("unix",
		cmdCtxt.httpCacheCtxt.Config.Server.DiagHost); err != nil {

		return

	}

	return
}

func (cmdCtxt *CommandCtxt) switchDiagnose(status bool) (output string, err error) {

	if cmdCtxt.httpCacheCtxt.Config.Server.Diagnose != status {
		swLock := &sync.RWMutex{}
		swLock.Lock()
		defer swLock.Unlock()

		output = fmt.Sprintf("Switching Diagnose status %t", status)
		cmdCtxt.httpCacheCtxt.Config.Server.Diagnose = status
	}
	return
}

func (cmdCtxt *CommandCtxt) dumpStats() (output string, err error) {

	var (
		fileB []byte
	)

	if fileB, err = json.Marshal(cmdCtxt.httpCacheCtxt.CpApiStats); err != nil {
		err = errors.New("Error in marshaling response")
		return
	}

	if err = ioutil.WriteFile(DumpFile, fileB, 0755); err != nil {
		err = errors.New("Error in writing response")
		return
	}

	output = fmt.Sprintf("Response has been dumped to %s", DumpFile)

	return
}

func (cmdCtxt *CommandCtxt) showSummaryStats() (output string, err error) {

	var (
		allItems         []ReqKeyT
		apiInvalidations uint32
	)

	if allItems, err = cmdCtxt.httpCacheCtxt.getActiveItems(); err != nil {
		err = errors.New("Error in fetching items")
		return
	}

	cmdCtxt.httpCacheCtxt.CpStats.Range(func(k1 interface{}, v1 interface{}) bool {

		var (
			statMap   *sync.Map
			statVal   interface{}
			isPresent bool
		)

		statMap = v1.(*sync.Map)

		if statVal, isPresent = statMap.Load(CacheInvalidationsStatKey); !isPresent {
			return true
		}

		apiInvalidations += statVal.(uint32)
		return true
	})

	output += fmt.Sprintf("Total Items         - %d\n", len(allItems))
	output += fmt.Sprintf("Total Invalidations - %d\n", apiInvalidations)
	output += fmt.Sprintf("Total RPS           - %d\n", cmdCtxt.httpCacheCtxt.SysStats.Rps)

	return
}

func (cmdCtxt *CommandCtxt) showCacheStats() (output string, err error) {

	var (
		cps []ReqKeyT

		totalCacheHit  uint32
		totalCacheMiss uint32
	)

	if cps, err = cmdCtxt.httpCacheCtxt.getActiveItems(); err != nil {
		return
	}

	for _, cp := range cps {

		var (
			apis []string
		)

		if apis, err = cmdCtxt.httpCacheCtxt.getApisOfItem(cp); err != nil {
			continue
		}

		for _, api := range apis {

			var (
				isPresent bool

				cacheHitIntf  interface{}
				cacheMissIntf interface{}

				apiMap     *sync.Map
				apiMapIntf interface{}
			)

			if apiMapIntf, isPresent = cmdCtxt.httpCacheCtxt.CpApiStats.Load(api); !isPresent {
				continue
			}

			apiMap = apiMapIntf.(*sync.Map)

			if cacheHitIntf, isPresent = apiMap.Load(CacheHitStatKey); !isPresent {
				continue
			}

			totalCacheHit += cacheHitIntf.(uint32)

			if cacheMissIntf, isPresent = apiMap.Load(CacheMissStatKey); !isPresent {
				continue
			}
			totalCacheMiss += cacheMissIntf.(uint32)
		}
	}

	output += fmt.Sprintf("Total Cache Hit  - %d\n", totalCacheHit)
	output += fmt.Sprintf("Total Cache Miss - %d\n", totalCacheMiss)

	return
}

func (cmdCtxt *CommandCtxt) showProxyStats() (output string, err error) {

	var (
		cps []ReqKeyT

		totalProxyReqRecvd  uint32
		totalProxyReqSent   uint32
		totalProxyRespRecvd uint32
	)

	if cps, err = cmdCtxt.httpCacheCtxt.getActiveItems(); err != nil {
		return
	}

	for _, cp := range cps {

		var (
			apis []string
		)

		if apis, err = cmdCtxt.httpCacheCtxt.getApisOfItem(cp); err != nil {
			continue
		}

		for _, api := range apis {

			var (
				isPresent          bool
				proxyReqRecvdIntf  interface{}
				proxyReqSentIntf   interface{}
				proxyRespRecvdIntf interface{}

				apiMap     *sync.Map
				apiMapIntf interface{}
			)

			if apiMapIntf, isPresent = cmdCtxt.httpCacheCtxt.CpApiStats.Load(api); !isPresent {
				continue
			}

			apiMap = apiMapIntf.(*sync.Map)

			if proxyReqRecvdIntf, isPresent = apiMap.Load(ProxyRequestRecvdStatKey); !isPresent {
				continue
			}

			totalProxyReqRecvd += proxyReqRecvdIntf.(uint32)

			if proxyReqSentIntf, isPresent = apiMap.Load(ProxyRequestSentStatKey); !isPresent {
				continue
			}

			totalProxyReqSent += proxyReqSentIntf.(uint32)

			if proxyRespRecvdIntf, isPresent = apiMap.Load(ProxyResponseRecvdStatKey); !isPresent {
				continue
			}

			totalProxyRespRecvd += proxyRespRecvdIntf.(uint32)
		}
	}

	output += fmt.Sprintf("Total Proxy Request Received  - %d\n", totalProxyReqRecvd)
	output += fmt.Sprintf("Total Proxy Request Sent      - %d\n", totalProxyReqSent)
	output += fmt.Sprintf("Total Proxy Response Received - %d\n", totalProxyRespRecvd)

	return
}

func (cmdCtxt *CommandCtxt) showBEStats() (output string, err error) {

	var (
		cps []ReqKeyT

		apiBeFails map[string]uint32
	)

	apiBeFails = make(map[string]uint32)

	if cps, err = cmdCtxt.httpCacheCtxt.getActiveItems(); err != nil {
		return
	}

	for _, cp := range cps {

		var (
			apis []string
		)

		if apis, err = cmdCtxt.httpCacheCtxt.getApisOfItem(cp); err != nil {
			continue
		}

		for _, api := range apis {

			var (
				isPresent bool

				beFailIntf interface{}
				beFail     uint32

				apiMap     *sync.Map
				apiMapIntf interface{}

				apiStatVal uint32
			)

			if apiMapIntf, isPresent = cmdCtxt.httpCacheCtxt.CpApiStats.Load(api); !isPresent {
				continue
			}

			apiMap = apiMapIntf.(*sync.Map)

			if beFailIntf, isPresent = apiMap.Load(BackendFailStatKey); !isPresent {
				continue
			}

			beFail = beFailIntf.(uint32)

			if apiStatVal, isPresent = apiBeFails[api]; !isPresent {
				apiBeFails[api] = 0
			}

			apiStatVal += beFail
			apiBeFails[api] = apiStatVal

		}
	}

	return
}

func (cmdCtxt *CommandCtxt) showHelpStats() (output string, err error) {

	output += "Following commands are available :\n\n"
	output += fmt.Sprintf("%s\n", ShowSummaryCommand)
	output += fmt.Sprintf("%s\n", ShowCacheStatsCommand)
	output += fmt.Sprintf("%s\n", ShowProxyStatsCommand)
	output += fmt.Sprintf("%s\n", ShowBEStatsCommand)
	output += fmt.Sprintf("%s\n", DumpStatsCommand)

	return
}

func (cmdCtxt *CommandCtxt) runCommand(cmd string) (output string, err error) {

	var (
		spCmd []string
	)

	spCmd = strings.Split(strings.TrimSpace(cmd), " ")

	if len(spCmd) == 0 {
		err = errors.New("No Input specified")

	} else {

		switch spCmd[0] {

		case TurnOnCommand:
			output, err = cmdCtxt.switchDiagnose(true)

		case TurnOffCommand:
			output, err = cmdCtxt.switchDiagnose(false)

		case DumpStatsCommand:
			output, err = cmdCtxt.dumpStats()

		case ShowSummaryCommand:
			output, err = cmdCtxt.showSummaryStats()

		case ShowCacheStatsCommand:
			output, err = cmdCtxt.showCacheStats()

		case ShowProxyStatsCommand:
			output, err = cmdCtxt.showProxyStats()

		case ShowBEStatsCommand:
			output, err = cmdCtxt.showBEStats()

		default:
			output, err = cmdCtxt.showHelpStats()

		}
	}

	if err != nil {
		output = err.Error()
	}

	output = fmt.Sprintf("\n\n\n%s\n%s\n%s\n\n\n", LineDelimiter, output, LineDelimiter)

	return
}

func (cmdCtxt *CommandCtxt) startSession(conn net.Conn) (err error) {

	cmdCtxt.switchDiagnose(true)

	for {

		var (
			buf       [1024]byte
			cmd       string
			output    string
			noOfBytes int
		)

		conn.Write([]byte(">> "))

		noOfBytes, err = conn.Read(buf[:])
		cmd = string(buf[:noOfBytes])

		log.Println("Received --> " + cmd)

		if output, err = cmdCtxt.runCommand(cmd); err != nil {
			output = "Error in running command --> " + cmd + "\nError --> " + err.Error() + "\n"
			log.Println(err)
		}

		if _, err = conn.Write([]byte(output)); err != nil {
			log.Println(err)
			return
		}

	}

	cmdCtxt.switchDiagnose(false)

	return
}

// Command format
// fetch_stats cp api stat
func (cmdCtxt *CommandCtxt) Process() (err error) {

	for {

		if err = cmdCtxt.startListening(); err == nil {
			log.Println(err)
			break
		}

		time.Sleep(time.Second * 10)
	}

	for {

		var (
			conn net.Conn
		)

		if conn, err = cmdCtxt.listener.Accept(); err != nil {
			log.Println(err)
			continue
		}

		go cmdCtxt.startSession(conn)
	}

	return
}
