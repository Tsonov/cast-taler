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
	// Store previous counter values to detect new traffic
	previousCounters map[string]float64
}

// NewOptimizer creates a new instance of Optimizer
func NewOptimizer(scraper *PrometheusScraper, pollInterval time.Duration, executor *BashExecutor, buoyantLicense string) *Optimizer {
	return &Optimizer{
		scraper:          scraper,
		pollInterval:     pollInterval,
		bash:             executor,
		buoyantLicense:   buoyantLicense,
		previousCounters: make(map[string]float64),
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

	// Create a map to store current counter values
	currentCounters := make(map[string]float64)

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

		// Create a unique key for this metric
		metricKey := fmt.Sprintf("%s:%s:%s:%s", sourceAZ, targetAZ, sourcePod, targetPod)

		// Store the current value
		currentCounters[metricKey] = value

		// Check if this is cross-AZ traffic
		if sourceAZ != targetAZ {
			// Get the previous value (if any)
			previousValue, exists := o.previousCounters[metricKey]
			if !exists {
				fmt.Println("First time we see this metric, setting previous value to 0")
				previousValue = 0
			}

			// If this is the first time we see this metric or if there's new traffic
			if value > previousValue {
				delta := value
				delta = value - previousValue

				fmt.Printf("NEW Cross-AZ traffic detected: source_az=%s, target_az=%s,  src=%s, target=%s, previous=%f, current=%f, delta=%f\n",
					sourceAZ, targetAZ, sourcePod, targetPod, previousValue, value, delta)
				result = append(result, CrossAZTraffic{
					SourcePod: sourcePod,
					TargetPod: targetPod,
				})
			} else {
				fmt.Printf("No new cross-AZ traffic: source_az=%s, target_az=%s, previous=%f, current=%f, src=%s, target=%s\n",
					sourceAZ, targetAZ, previousValue, value, sourcePod, targetPod)
			}
		} else {
			fmt.Printf("Non-cross-AZ traffic: source_az=%s, target_az=%s, value=%f\n", sourceAZ, targetAZ, value)
		}
	}

	// Update the previous counters for the next cycle
	o.previousCounters = currentCounters

	return result
}

func (o *Optimizer) optimize(traffic []CrossAZTraffic) error {
	fmt.Println("Optimizing...")

	fmt.Println("Creating pod-mutations for TSC")
	if err := o.bash.ExecuteScriptStreaming("../hack/topologyspread/pod-mutator.sh", nil, map[string]string{
		"CASTAI_API_URI":   "api.cast.ai",
		"ORGANIZATION_ID":  "8c39f55e-4710-4cb7-b106-3f3300818c69",
		"CLUSTER_ID":       "197bfadc-ca34-4c50-a23f-1236394c8558",
		"CASTAI_API_TOKEN": "0b229ebf5acb6b972656628116531034021fdb3b3fea52f775786793abbfe3b0",
	}); err != nil {
		return fmt.Errorf("failed to create TSC pod-mutations, script error: %w", err)
	}

	fmt.Println("Pod-mutations for TSC created")

	fmt.Println("Creating pod-mutations for HAZL")
	// This goes second as it will force the pod mutations to be applied AND restart so might as well.
	if err := o.bash.ExecuteScriptStreaming("../hack/linkerd/pod-mutator.sh", nil, map[string]string{
		"CASTAI_API_URI":   "api.cast.ai",
		"ORGANIZATION_ID":  "8c39f55e-4710-4cb7-b106-3f3300818c69",
		"CLUSTER_ID":       "197bfadc-ca34-4c50-a23f-1236394c8558",
		"CASTAI_API_TOKEN": "0b229ebf5acb6b972656628116531034021fdb3b3fea52f775786793abbfe3b0",
	}); err != nil {
		return fmt.Errorf("failed to create HAZL pod-mutations, script error: %w", err)
	}
	fmt.Println("Pod-mutations for HAZL created")

	fmt.Println("Installing HAZL...")
	if err := o.bash.ExecuteScriptStreaming("../hack/linkerd/hazl-enable.sh", nil, map[string]string{
		"LINKERD_CMD":     "/Users/lachezar/.linkerd2/bin/linkerd",
		"BUOYANT_LICENSE": o.buoyantLicense,
	}); err != nil {
		return fmt.Errorf("failed to enable hazl, script error: %w", err)
	}
	fmt.Println("Installed HAZL")

	fmt.Println("Optimizing done")
	return nil
}
