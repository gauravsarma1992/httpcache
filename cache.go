package httpcache

import (
	"errors"
	"log"
	"sync"
	"time"
)

type (
	Cache struct {
		httpCacheCtxt *HttpCacheCtxt

		//CacheObj map[ReqKeyT]*CacheObj
		CacheObj sync.Map
	}

	CacheObj struct {
		IsValid bool

		// CachedAt is used to determine
		// when the cache invalidation request
		// was received in the service.
		// UpdatedAt is used to determine
		// the time when the response received
		// from the backend is formed and stored
		// in the cache
		CachedAt int64

		CacheApi map[string]*CacheApi
	}

	CacheApi struct {
		Base *CacheObj

		UpdatedAt int64
		Data      []byte
	}
)

func NewCache(httpCacheCtxt *HttpCacheCtxt) (cache *Cache, err error) {

	cache = &Cache{
		httpCacheCtxt: httpCacheCtxt,
	}

	return
}

func (cache *Cache) GetData(reqKey ReqKeyT, apiName string) (respBody []byte, err error) {

	var (
		cacheApi *CacheApi
	)

	if cacheApi, err = cache.get(reqKey, apiName); err != nil {
		err = errors.New("No Cache Found for key " + string(reqKey))
		return
	}

	respBody = cacheApi.Data

	return
}

func (cache *Cache) get(reqKey ReqKeyT, apiName string) (cacheApi *CacheApi, err error) {

	var (
		isPresent bool
		cacheObj  *CacheObj
		cacheIntf interface{}
	)

	cacheIntf, _ = cache.CacheObj.LoadOrStore(reqKey, &CacheObj{})

	cacheObj = cacheIntf.(*CacheObj)

	if cacheApi, isPresent = cacheObj.CacheApi[apiName]; !isPresent {
		err = errors.New("No Cache Found for key " + string(reqKey))
		return
	}

	return
}

func (cache *Cache) Add(reqKey ReqKeyT, apiName string, respBody []byte) (err error) {

	var (
		swLock *sync.RWMutex

		cacheObj  *CacheObj
		cacheApi  *CacheApi
		cacheIntf interface{}

		currTime int64
	)

	currTime = time.Now().Unix()

	swLock = &sync.RWMutex{}
	swLock.Lock()
	defer swLock.Unlock()

	cacheIntf, _ = cache.CacheObj.LoadOrStore(reqKey, &CacheObj{})
	cacheObj = cacheIntf.(*CacheObj)

	if cacheObj.CacheApi == nil {
		cacheObj.CacheApi = make(map[string]*CacheApi)
	}

	if cacheApi, err = cache.get(reqKey, apiName); err != nil {
		cacheApi = &CacheApi{
			Base: cacheObj,
		}
		cacheObj.CacheApi[apiName] = cacheApi
	}

	cacheObj.CachedAt = currTime

	cacheApi.UpdatedAt = currTime
	cacheApi.Data = respBody

	return
}

func (cache *Cache) Invalidate(reqKey ReqKeyT) (err error) {

	var (
		swLock    *sync.RWMutex
		cacheObj  *CacheObj
		cacheIntf interface{}

		isPresent bool
	)

	log.Println("Invalidating cache for", reqKey)

	if cacheIntf, isPresent = cache.CacheObj.Load(reqKey); !isPresent {
		return
	}

	cacheObj = cacheIntf.(*CacheObj)

	swLock = &sync.RWMutex{}
	swLock.Lock()
	defer swLock.Unlock()

	cacheObj.CachedAt = time.Now().Unix()

	return
}

func (cache *Cache) IsValid(reqKey ReqKeyT, apiName string) (isValid bool) {

	var (
		cacheApi *CacheApi
		err      error
	)

	if cacheApi, err = cache.get(reqKey, apiName); err != nil {
		isValid = false
		return
	}

	if cacheApi.Base.CachedAt == cacheApi.UpdatedAt {
		isValid = true
		return
	}

	return
}
