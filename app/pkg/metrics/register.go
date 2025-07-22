package metrics

func RegisterCustomMetrics() {
	registry.MustRegister(trafficCounter)
}
