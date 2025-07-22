package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var trafficCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "traffic_total",
		Help: "Total bytes sent and received by protocol, source pod, source az, target az, target pod, and success.",
	},
	[]string{"success", "protocol", "source_pod", "source_az", "target_az", "target_pod"})

func TrackTraffic(bytes float64, success bool, protocol string, sourcePod string, sourceAz string, targetAz string, targetName string) {
	trafficCounter.With(prometheus.Labels{
		"success":    strconv.FormatBool(success),
		"protocol":   protocol,
		"source_pod": sourcePod,
		"source_az":  sourceAz,
		"target_az":  targetAz,
		"target_pod": targetName,
	}).Add(bytes)
}
