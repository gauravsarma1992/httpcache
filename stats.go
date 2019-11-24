package httpcache

import (
	"sync"
	"sync/atomic"
)

const (
	TimeFrequencyLimit = 1000

	CacheMissStatKey   = uint32(0)
	CacheHitStatKey    = uint32(1)
	BackendFailStatKey = uint32(2)
	LocalCacheStatKey  = uint32(3)

	RpsStatKey = uint32(4)

	CacheInvalidationsStatKey = uint32(5)
	LoginRequestsStatKey      = uint32(6)

	ProxyRequestRecvdStatKey  = uint32(7)
	ProxyRequestSentStatKey   = uint32(8)
	ProxyResponseRecvdStatKey = uint32(9)

	ProxyResponseTImeStatKey = uint32(10)
	CacheResponseTimeStatKey = uint32(11)
	BEResponseTimeStatKey    = uint32(12)
)

type (
	SysStats struct {
		Rps uint32
	}

	ResponseTime struct {
		Min int
		Max int
		Avg int

		Count   int
		History []int
	}
)

func (httpCacheCtxt *HttpCacheCtxt) regSysStats(statType uint32) (err error) {

	if httpCacheCtxt.Config.Server.Diagnose == false {
		return
	}

	switch statType {
	case RpsStatKey:
		httpCacheCtxt.SysStats.Rps = atomic.AddUint32(&httpCacheCtxt.SysStats.Rps, 1)
	}

	return
}

func (httpCacheCtxt *HttpCacheCtxt) regCpStats(reqKey ReqKeyT,
	statType uint32) (err error) {

	if httpCacheCtxt.Config.Server.Diagnose == false {
		return
	}

	mapIntf, _ := httpCacheCtxt.CpStats.LoadOrStore(reqKey, &sync.Map{})
	statMap := mapIntf.(*sync.Map)

	statValIntf, _ := statMap.LoadOrStore(statType, uint32(0))
	statVal := statValIntf.(uint32)

	statMap.Store(reqKey, statVal+1)

	return
}

func (httpCacheCtxt *HttpCacheCtxt) regCpApiTimeStats(reqKey ReqKeyT,
	apiName string, statType uint32) (err error) {

	if httpCacheCtxt.Config.Server.Diagnose == false {
		return
	}

	return
}

func (httpCacheCtxt *HttpCacheCtxt) regCpApiStats(reqKey ReqKeyT,
	apiName string, statType uint32) (err error) {

	if httpCacheCtxt.Config.Server.Diagnose == false {
		return
	}

	var (
		apiMap  *sync.Map
		statMap *sync.Map
		statVal uint32

		mapIntf interface{}
	)

	mapIntf, _ = httpCacheCtxt.CpApiStats.LoadOrStore(reqKey, &sync.Map{})
	apiMap = mapIntf.(*sync.Map)

	mapIntf, _ = apiMap.LoadOrStore(apiName, &sync.Map{})
	statMap = mapIntf.(*sync.Map)

	mapIntf, _ = statMap.LoadOrStore(statType, uint32(0))
	statVal = mapIntf.(uint32)

	statMap.Store(statType, statVal+1)

	apiMap.Store(apiName, statMap)
	httpCacheCtxt.CpApiStats.Store(reqKey, apiMap)

	return
}
