package main

import (
	"fmt"
	"time"

	dto "github.com/prometheus/client_model/go"
)

const (
	TrafficTotalMetricName = "traffic_total"
	LabelSourceAz          = "source_az"
	LabelTargetAz          = "target_az"
	LabelSourcePod         = "source_pod"
	LabelTargetPod         = "target_pod"
)

// Optimizer is responsible for analyzing Prometheus metrics and identifying cross-AZ traffic
type Optimizer struct {
	scraper      *PrometheusScraper
	pollInterval time.Duration
}

// NewOptimizer creates a new instance of Optimizer
func NewOptimizer(scraper *PrometheusScraper, pollInterval time.Duration) *Optimizer {
	return &Optimizer{
		scraper:      scraper,
		pollInterval: pollInterval,
	}
}

// Run starts the optimizer's main loop
func (o *Optimizer) Run() {
	fmt.Println("Starting optimizer...")

	for {
		o.analyzeTrafficMetrics()
		fmt.Println("Optimizer cycle done, sleeping for", o.pollInterval)
		time.Sleep(o.pollInterval)
	}
}

// analyzeTrafficMetrics scrapes and analyzes traffic metrics
func (o *Optimizer) analyzeTrafficMetrics() {
	metrics, err := o.scraper.ScrapeMetrics()
	if err != nil {
		fmt.Printf("Error scraping metrics: %v\n", err)
		return
	}

	// Look for the traffic_total metric family
	family, exists := metrics[TrafficTotalMetricName]
	if !exists {
		fmt.Printf("Metric family %s not found\n", TrafficTotalMetricName)
		return
	}

	fmt.Printf("Analyzing %d metrics in family %s\n", len(family.GetMetric()), TrafficTotalMetricName)

	// Iterate through metrics in the family
	for _, metric := range family.GetMetric() {
		var sourceAZ, targetAZ string
		var value float64

		// Extract labels
		for _, label := range metric.GetLabel() {
			if label.GetName() == LabelSourceAz {
				sourceAZ = label.GetValue()
			} else if label.GetName() == LabelTargetAz {
				targetAZ = label.GetValue()
			}
		}

		// Get the metric value based on its type
		switch family.GetType() {
		case dto.MetricType_COUNTER:
			value = metric.GetCounter().GetValue()
		default:
			// Skip other metric types
			fmt.Printf("Unsupported metric type: %s\n", family.GetType().String())
			continue
		}

		// Check if this is cross-AZ traffic with positive value
		if sourceAZ != targetAZ && value > 0 {
			fmt.Printf("Cross-AZ traffic detected: source_az=%s, target_az=%s, value=%f\n",
				sourceAZ, targetAZ, value)
		} else {
			fmt.Printf("Non-cross-AZ traffic detected: source_az=%s, target_az=%s, value=%f\n", sourceAZ, targetAZ, value)
		}
	}
}
