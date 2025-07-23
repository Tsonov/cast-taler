package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// PrometheusScraper is responsible for scraping metrics from a Prometheus instance
type PrometheusScraper struct {
	URL     string
	Timeout time.Duration
	client  *http.Client
	isAPI   bool // true if URL points to a Prometheus API, false if it's a direct metrics endpoint
}

// NewPrometheusScraper creates a new instance of PrometheusScraper
func NewPrometheusScraper(url string, timeout time.Duration, isAPI bool) *PrometheusScraper {
	return &PrometheusScraper{
		URL:     url,
		Timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		isAPI: isAPI,
	}
}

// ScrapeMetrics fetches metrics from the configured Prometheus endpoint or API
func (s *PrometheusScraper) ScrapeMetrics() (map[string]*dto.MetricFamily, error) {
	if s.isAPI {
		return s.scrapeFromPrometheusAPI()
	} else {
		return s.scrapeFromEndpoint()
	}
}

// scrapeFromEndpoint fetches metrics directly from a metrics endpoint
func (s *PrometheusScraper) scrapeFromEndpoint() (map[string]*dto.MetricFamily, error) {
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

// PrometheusQueryResponse represents the response structure from Prometheus API
type PrometheusQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// scrapeFromPrometheusAPI fetches metrics from a Prometheus API instance
func (s *PrometheusScraper) scrapeFromPrometheusAPI() (map[string]*dto.MetricFamily, error) {
	// Construct the API URL for querying metrics
	apiURL, err := url.Parse(s.URL)
	if err != nil {
		return nil, fmt.Errorf("error parsing Prometheus API URL: %v", err)
	}

	// Ensure the path ends with /api/v1/query
	if apiURL.Path == "" || apiURL.Path == "/" {
		apiURL.Path = "/api/v1/query"
	} else if !strings.HasSuffix(apiURL.Path, "/api/v1/query") {
		apiURL.Path = apiURL.Path + "/api/v1/query"
	}

	// Add query parameter for the traffic_total metric
	q := apiURL.Query()
	q.Set("query", TrafficTotalMetricName)
	apiURL.RawQuery = q.Encode()

	// Make the request to the Prometheus API
	resp, err := s.client.Get(apiURL.String())
	if err != nil {
		return nil, fmt.Errorf("error making request to Prometheus API %s: %v", apiURL.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-OK response status from Prometheus API: %s", resp.Status)
	}

	// Read and parse the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	var queryResponse PrometheusQueryResponse
	if err := json.Unmarshal(body, &queryResponse); err != nil {
		return nil, fmt.Errorf("error parsing Prometheus API response: %v", err)
	}

	// Convert the Prometheus API response to MetricFamily format
	metrics := make(map[string]*dto.MetricFamily)
	metricName := TrafficTotalMetricName
	helpText := ""
	family := &dto.MetricFamily{
		Name:   &metricName,
		Type:   dto.MetricType_COUNTER.Enum(),
		Help:   &helpText,
		Metric: []*dto.Metric{},
	}

	for _, result := range queryResponse.Data.Result {
		metric := &dto.Metric{
			Label:   []*dto.LabelPair{},
			Counter: &dto.Counter{},
		}

		// Extract labels
		for name, value := range result.Metric {
			labelName := name
			labelValue := value
			metric.Label = append(metric.Label, &dto.LabelPair{
				Name:  &labelName,
				Value: &labelValue,
			})
		}

		// Extract value
		if len(result.Value) >= 2 {
			if strValue, ok := result.Value[1].(string); ok {
				if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil {
					metric.Counter.Value = &floatValue
				}
			}
		}

		family.Metric = append(family.Metric, metric)
	}

	metrics[TrafficTotalMetricName] = family
	return metrics, nil
}

// DisplayMetrics prints all metrics to stdout
func (s *PrometheusScraper) DisplayMetrics() error {
	metrics, err := s.ScrapeMetrics()
	if err != nil {
		return err
	}

	if s.isAPI {
		fmt.Printf("Scraped metrics from Prometheus API at %s\n", s.URL)
	} else {
		fmt.Printf("Scraped metrics from endpoint at %s\n", s.URL)
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
