package httpcache

import (
	"errors"
	"sync"
)

type (
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
		caLock   *sync.RWMutex
	}

	CacheApi struct {
		Base *CacheObj

		UpdatedAt int64
		Data      []byte
	}
)

func NewCacheObj() (cacheObj *CacheObj, err error) {

	cacheObj = &CacheObj{
		CacheApi: make(map[string]*CacheApi),
		caLock:   &sync.RWMutex{},
	}

	return
}

func (cacheObj *CacheObj) GetOrCreateCacheApi(apiName string) (cacheApi *CacheApi, err error) {

	if cacheApi, err = cacheObj.GetCacheApi(apiName); err != nil {

		cacheApi = &CacheApi{}

		cacheObj.caLock.Lock()
		cacheObj.CacheApi[apiName] = cacheApi
		cacheObj.caLock.Unlock()
	}

	return
}

func (cacheObj *CacheObj) GetCacheApi(apiName string) (cacheApi *CacheApi, err error) {

	var (
		isPresent bool
	)

	if cacheApi, isPresent = cacheObj.CacheApi[apiName]; !isPresent {
		err = errors.New("No Cache Found for key " + string(apiName))
		return
	}

	return
}

func (cacheApi *CacheApi) IsValid() (isValid bool) {

	if cacheApi.Base.CachedAt == cacheApi.UpdatedAt {
		isValid = true
		return
	}

	return
}
