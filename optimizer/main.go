package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/util/homedir"
)

func main() {
	// Define command-line flags
	var kubeconfig string
	var kubecontext string
	var prometheusURL string
	var prometheusTimeout time.Duration
	var prometheusIsAPI bool
	var buoyantLicense string

	// Set default kubeconfig path if not specified
	if home := homedir.HomeDir(); home != "" {
		pflag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"),
			"(optional) absolute path to the kubeconfig file")
	} else {
		pflag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
	pflag.StringVar(&kubecontext, "kubecontext", "", "name of the kubeconfig context to use")

	// Prometheus scraper flags
	pflag.StringVar(&prometheusURL, "prometheus-url", "", "URL of the Prometheus metrics endpoint or API to scrape")
	pflag.DurationVar(&prometheusTimeout, "prometheus-timeout", 10*time.Second, "Timeout for Prometheus metrics scraping")
	pflag.BoolVar(&prometheusIsAPI, "prometheus-is-api", false, "Set to true if prometheus-url points to a Prometheus API instance instead of a direct metrics endpoint")

	// Buoyant license flag
	pflag.StringVar(&buoyantLicense, "buoyant-license", "", "Buoyant license key required for Linkerd")

	// Linkerd command flag
	var linkerdCmd string
	pflag.StringVar(&linkerdCmd, "linkerd-cmd", "~/.linkerd2/bin/linkerd", "Path to the Linkerd CLI command")

	// CASTAI config flags
	var castaiAPIURI string
	var castaiOrgID string
	var castaiClusterID string
	var castaiAPIToken string
	pflag.StringVar(&castaiAPIURI, "castai-api-uri", "", "CASTAI API URI")
	pflag.StringVar(&castaiOrgID, "castai-org-id", "", "CASTAI Organization ID")
	pflag.StringVar(&castaiClusterID, "castai-cluster-id", "", "CASTAI Cluster ID")
	pflag.StringVar(&castaiAPIToken, "castai-api-token", "", "CASTAI API Token")

	pflag.Parse()

	// Validate required flags
	if prometheusURL == "" {
		fmt.Println("Error: --prometheus-url is required")
		os.Exit(1)
	}
	if castaiAPIURI == "" || castaiOrgID == "" || castaiClusterID == "" || castaiAPIToken == "" {
		fmt.Println("Error: all CASTAI configuration flags are required")
		os.Exit(1)
	}

	// Your optimizer logic goes here
	fmt.Println("Optimizer started")

	// If Prometheus URL is provided, create and run the optimizer
	if prometheusURL != "" {
		if prometheusIsAPI {
			fmt.Printf("Connecting to Prometheus API at %s\n", prometheusURL)
		} else {
			fmt.Printf("Connecting to Prometheus metrics endpoint at %s\n", prometheusURL)
		}
		scraper := NewPrometheusScraper(prometheusURL, prometheusTimeout, prometheusIsAPI)
		executor := NewBashExecutor()
		// Enable streaming output by default
		executor.SetStreamOutput(true)

		config := OptimizerConfig{
			PollInterval:   10 * time.Second,
			BuoyantLicense: buoyantLicense,
			CastaiConfig: CastaiConfig{
				ApiUri:         castaiAPIURI,
				OrganizationId: castaiOrgID,
				ClusterId:      castaiClusterID,
				ApiToken:       castaiAPIToken,
			},
			LinkerdCmd: linkerdCmd,
		}

		optimizer := NewOptimizer(scraper, executor, config)

		// Run the optimizer (this will block indefinitely)
		optimizer.Run()
	}
}
