package httpcache

import (
	"log"
	"net/http"
)

func (httpCacheCtxt *HttpCacheCtxt) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		log.Println(r.URL.Path)
		next.ServeHTTP(w, r)

	})
}
