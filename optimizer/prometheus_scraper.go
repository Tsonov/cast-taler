package main

import (
	"fmt"
	"net/http"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// PrometheusScraper is responsible for scraping metrics from a Prometheus endpoint
type PrometheusScraper struct {
	URL     string
	Timeout time.Duration
	client  *http.Client
}

// NewPrometheusScraper creates a new instance of PrometheusScraper
func NewPrometheusScraper(url string, timeout time.Duration) *PrometheusScraper {
	return &PrometheusScraper{
		URL:     url,
		Timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// ScrapeMetrics fetches metrics from the configured Prometheus endpoint
func (s *PrometheusScraper) ScrapeMetrics() (map[string]*dto.MetricFamily, error) {
	// Make HTTP request to the metrics endpoint
	resp, err := s.client.Get(s.URL)
	if err != nil {
		return nil, fmt.Errorf("error making request to %s: %v", s.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK response status: %s", resp.Status)
	}

	// Parse the response body as Prometheus metrics
	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing metrics: %v", err)
	}

	return metrics, nil
}

// DisplayMetrics prints all metrics to stdout
func (s *PrometheusScraper) DisplayMetrics() error {
	metrics, err := s.ScrapeMetrics()
	if err != nil {
		return err
	}

	fmt.Printf("Found %d metric families\n", len(metrics))
	for name, family := range metrics {
		fmt.Printf("\nMetric Family: %s\n", name)
		fmt.Printf("Type: %s\n", family.GetType().String())
		fmt.Printf("Help: %s\n", family.GetHelp())

		for _, metric := range family.GetMetric() {
			fmt.Printf("  Labels: ")
			for _, label := range metric.GetLabel() {
				fmt.Printf("%s=%s ", label.GetName(), label.GetValue())
			}

			switch family.GetType() {
			case dto.MetricType_COUNTER:
				fmt.Printf("Value: %v\n", metric.GetCounter().GetValue())
			case dto.MetricType_GAUGE:
				fmt.Printf("Value: %v\n", metric.GetGauge().GetValue())
			case dto.MetricType_HISTOGRAM:
				h := metric.GetHistogram()
				fmt.Printf("Count: %v, Sum: %v\n", h.GetSampleCount(), h.GetSampleSum())
			case dto.MetricType_SUMMARY:
				s := metric.GetSummary()
				fmt.Printf("Count: %v, Sum: %v\n", s.GetSampleCount(), s.GetSampleSum())
			}
		}
	}

	return nil
}
