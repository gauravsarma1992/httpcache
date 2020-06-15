package httpcache

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type (
	Stats struct {
		Counter struct {
			Invalidations  prometheus.Counter
			Requests       prometheus.Counter
			Proxied        prometheus.Counter
			Skipped        prometheus.Counter
			LocalHandled   prometheus.Counter
			CacheAdded     prometheus.Counter
			CachedResponse prometheus.Counter
		}
	}
)

func NewMonitorServer() (server *http.Server, err error) {

	var (
		router *mux.Router
	)

	router = mux.NewRouter()

	router.Handle("/metrics", promhttp.Handler())

	server = &http.Server{
		Handler:      router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	return
}

func NewStats() (stats *Stats, err error) {

	stats = &Stats{}

	if err = stats.RegisterCounterStats(); err != nil {
		return
	}

	return
}

func (stats *Stats) RegisterCounterStats() (err error) {

	stats.Counter.Invalidations = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_invalidatons"})
	stats.Counter.Requests = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_requests"})
	stats.Counter.Proxied = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_proxied"})
	stats.Counter.Skipped = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_skipped"})
	stats.Counter.LocalHandled = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_local_handled"})
	stats.Counter.CacheAdded = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_added"})
	stats.Counter.CachedResponse = prometheus.NewCounter(prometheus.CounterOpts{Name: "cache_response"})

	prometheus.MustRegister(stats.Counter.Invalidations)
	prometheus.MustRegister(stats.Counter.Requests)
	prometheus.MustRegister(stats.Counter.Proxied)
	prometheus.MustRegister(stats.Counter.Skipped)
	prometheus.MustRegister(stats.Counter.LocalHandled)
	prometheus.MustRegister(stats.Counter.CacheAdded)
	prometheus.MustRegister(stats.Counter.CachedResponse)

	return
}
