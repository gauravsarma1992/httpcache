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

		CacheObj map[ReqKeyT]*CacheObj
		objLock  *sync.RWMutex

		//CacheObj sync.Map
	}
)

func NewCache(httpCacheCtxt *HttpCacheCtxt) (cache *Cache, err error) {

	cache = &Cache{
		httpCacheCtxt: httpCacheCtxt,

		CacheObj: make(map[ReqKeyT]*CacheObj, 1024),
		objLock:  &sync.RWMutex{},
	}

	return
}

func (cache *Cache) getCacheObj(reqKey ReqKeyT) (cacheObj *CacheObj, err error) {

	var (
		isPresent bool
	)

	cache.objLock.RLock()
	defer cache.objLock.RUnlock()

	if cacheObj, isPresent = cache.CacheObj[reqKey]; !isPresent {
		err = errors.New("No CloudPort Found for key " + string(reqKey))
		return
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
		cacheObj *CacheObj
	)

	if cacheObj, err = cache.getCacheObj(reqKey); err != nil {
		err = errors.New("No CloudPort Found for key " + string(reqKey))
		return
	}

	if cacheApi, err = cacheObj.GetCacheApi(apiName); err != nil {
		return
	}

	return
}

func (cache *Cache) Add(reqKey ReqKeyT, apiName string, respBody []byte) (err error) {

	var (
		cacheObj *CacheObj
		cacheApi *CacheApi

		currTime int64
	)

	currTime = time.Now().Unix()

	if cacheObj, err = cache.getCacheObj(reqKey); err != nil {

		// Initialize the cache object per request element
		cacheObj, _ = NewCacheObj()

		cache.objLock.Lock()
		cache.CacheObj[reqKey] = cacheObj
		cache.objLock.Unlock()
	}

	if cacheApi, err = cacheObj.GetOrCreateCacheApi(apiName); err != nil {
		return
	}

	cacheObj.CachedAt = currTime

	cacheApi.UpdatedAt = currTime
	cacheApi.Data = respBody

	return
}

func (cache *Cache) Invalidate(reqKey ReqKeyT) (err error) {

	var (
		cacheObj *CacheObj
	)

	log.Println("Invalidating cache for", reqKey)

	if cacheObj, err = cache.getCacheObj(reqKey); err != nil {
		return
	}

	cacheObj.CachedAt = time.Now().Unix()

	return
}

func (cache *Cache) IsValid(reqKey ReqKeyT, apiName string) (isValid bool) {

	var (
		cacheObj *CacheObj
		cacheApi *CacheApi
		err      error
	)

	if cacheObj, err = cache.getCacheObj(reqKey); err != nil {
		return
	}

	if cacheApi, err = cacheObj.GetCacheApi(apiName); err != nil {
		isValid = false
		return
	}

	isValid = cacheApi.IsValid()

	return
}
