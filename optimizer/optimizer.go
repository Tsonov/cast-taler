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
	scraper        *PrometheusScraper
	pollInterval   time.Duration
	bash           *BashExecutor
	buoyantLicense string
}

// NewOptimizer creates a new instance of Optimizer
func NewOptimizer(scraper *PrometheusScraper, pollInterval time.Duration, executor *BashExecutor, buoyantLicense string) *Optimizer {
	return &Optimizer{
		scraper:        scraper,
		pollInterval:   pollInterval,
		bash:           executor,
		buoyantLicense: buoyantLicense,
	}
}

// Run starts the optimizer's main loop
func (o *Optimizer) Run() {
	fmt.Println("Starting optimizer...")

	for {
		crossAZTraffic := o.analyzeTrafficMetrics()
		if len(crossAZTraffic) > 0 {
			fmt.Println(fmt.Sprintf("detected %d instances of cross-AZ traffic", len(crossAZTraffic)))
			if err := o.optimize(crossAZTraffic); err != nil {
				fmt.Println("optimizer cycle failed, error: ", err)
			}
		}
		fmt.Println("Optimizer cycle done, sleeping for", o.pollInterval)
		time.Sleep(o.pollInterval)
	}
}

// CrossAZTraffic represents a pair of pods with cross-AZ traffic
type CrossAZTraffic struct {
	SourcePod string
	TargetPod string
}

// analyzeTrafficMetrics scrapes and analyzes traffic metrics
func (o *Optimizer) analyzeTrafficMetrics() []CrossAZTraffic {
	result := make([]CrossAZTraffic, 0)
	metrics, err := o.scraper.ScrapeMetrics()
	if err != nil {
		fmt.Printf("Error scraping metrics: %v\n", err)
		return result
	}

	// Look for the traffic_total metric family
	family, exists := metrics[TrafficTotalMetricName]
	if !exists {
		fmt.Printf("Metric family %s not found\n", TrafficTotalMetricName)
		return result
	}

	fmt.Printf("Analyzing %d metrics in family %s\n", len(family.GetMetric()), TrafficTotalMetricName)

	// Iterate through metrics in the family
	for _, metric := range family.GetMetric() {
		var sourceAZ, targetAZ, sourcePod, targetPod string
		var value float64

		// Extract labels
		for _, label := range metric.GetLabel() {
			switch label.GetName() {
			case LabelSourceAz:
				sourceAZ = label.GetValue()
			case LabelTargetAz:
				targetAZ = label.GetValue()
			case LabelSourcePod:
				sourcePod = label.GetValue()
			case LabelTargetPod:
				targetPod = label.GetValue()
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
			result = append(result, CrossAZTraffic{
				SourcePod: sourcePod,
				TargetPod: targetPod,
			})
		} else {
			fmt.Printf("Non-cross-AZ traffic detected: source_az=%s, target_az=%s, value=%f\n", sourceAZ, targetAZ, value)
		}
	}

	return result
}

func (o *Optimizer) optimize(traffic []CrossAZTraffic) error {
	fmt.Println("Optimizing...")
	// Enable linkerd (if needed)

	// Use the streaming version of ExecuteScript to see output in real-time
	fmt.Println("Installing Linkerd...")
	err := o.bash.ExecuteScriptStreaming("../hack/linkerd/install.sh", nil, map[string]string{
		"BUOYANT_LICENSE": o.buoyantLicense,
	})
	if err != nil {
		return fmt.Errorf("failed to install linkerd, script error: %w", err)
	}
	fmt.Println("Linkerd installed")

	return nil
}
