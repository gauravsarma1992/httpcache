# HttpCache Proxy

## Description

HttpCache Proxy is a proxy service sitting in front of
the HttpCache server backend (BE). It receives the requests
from any reverse proxy server(NGINX) and provides the response
by performing any of the following activities.

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
