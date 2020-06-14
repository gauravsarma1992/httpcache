# HttpCache Proxy

## Description

HttpCache Proxy is a proxy service sitting in front of
any server backend (BE).

The HttpCache Proxy can sit in front of the backend
server application. The proxy is mainly helpful if
we want to cache in the responses of any previous requests.

The proxy will maintain a cache of the responses per key
so that if the same key comes, the proxy can respond instead
of the backend server computing the entire thing again.

The proxy also exposes invalidation APIs which can be used
to invalidate the cache.

If the proxy doesn't have any cache mapping or if the cache is
invalidated, the proxy will itself send the request to the
backend, store the response in the cache and respond to the
request from its cache.

In case there are APIs for which we want to skip the cache
lookup, we can configure it in the same manner.

A commmon scenario will be to place the HttpCache proxy
in between the reverse proxy server (NGINX) and the actual
backend server application.

Following actions are supported:

- Respond from in-memory cache
- Skip cache and response from BE
- Local Handler which skips both
the cache and the BE

## Registering Local Handlers

To register a handler with the request processing
```go
if err = httpCacheCtxt.RegisterLocalCacheHandler("/something",
  someHandler); err != nil {

  log.Println(err)
  os.Exit(-1)
}
```

A sample handler
```go
func someHandler(w http.ResponseWriter,
	req *http.Request) (respBody []byte, err error) {

	respBody = []byte("{\"something\": \"some-response\"}")

	return
}
```

## Registering Middlewares

To register a middleware
```go
if err = httpCacheCtxt.RegisterMiddleware(someMiddleware); err != nil {

		log.Println(err)
		os.Exit(-1)
}
```

A sample middleware
```go
func someMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

        log.Println(r.URL.Path)
        next.ServeHTTP(w, r)

    })
}
```

Note - The middleware has to be set before the Process() function is called

## Statistics

### Run Stats CLI

```
nc -U /var/run/httpcache/sockets/diag.sock
```

### System Level

- No of requests per second

### Item Level

- Cache Invalidations
- Login requests

### Item Per API Level

- Active Items

- Cache Hit
- Cache Miss (Means the request has been forwarded to the BE)
- Cache Response Time

- Proxy Request received
- Proxy Request sent
- Proxy Request response received
- Proxy Response Time

- BE (Backend) failure
- BE Response Time

NOTE - All time related statistics should be in
- Min Time
- Max Time
- Avg Time

## Commands

- `help` to display the list of commands
- `turn-on-diag` to turn on the Diagnose module
- `turn-off-diag` to turn off the Diagnose module
- `dump-stats` to dump the statistics in /tmp/stats.json
- `show-summary` to show number of cloudports and number of APIs in cache,
nu`mber of cache invalidations, login requests
- `show-proxy-stats` to show the proxy request stats
- `show-cache-stats` to show the caching stats
- `show-backend-stats` to show the backend stats
