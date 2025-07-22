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
	[]string{"success", "protocol", "source-pod", "source-az", "target-az", "target-pod"})

func TrackTraffic(bytes float64, success bool, protocol string, sourcePod string, sourceAz string, targetAz string, targetName string) {
	trafficCounter.With(prometheus.Labels{
		"success":    strconv.FormatBool(success),
		"protocol":   protocol,
		"source-pod": sourcePod,
		"source-az":  sourceAz,
		"target-az":  targetAz,
		"target-pod": targetName,
	}).Add(bytes)
}
