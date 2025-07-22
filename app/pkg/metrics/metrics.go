package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registry = prometheus.NewRegistry()

func NewMetricsMux() *http.ServeMux {
	metricsMux := http.NewServeMux()

	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})

	return metricsMux
}
