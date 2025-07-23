package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func initKubeClient(kubeconfig, kubecontext string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error

	if kubeconfig != "" {
		// Use out-of-cluster configuration with the provided kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		loadingRules.ExplicitPath = kubeconfig

		configOverrides := &clientcmd.ConfigOverrides{}
		if kubecontext != "" {
			configOverrides.CurrentContext = kubecontext
		}

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("error creating Kubernetes client config from kubeconfig: %v", err)
		}
	} else {
		// Use in-cluster configuration
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("error creating in-cluster config: %v", err)
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes client: %v", err)
	}

	return clientset, nil
}

func main() {
	// Define command-line flags
	var kubeconfig string
	var kubecontext string
	var prometheusURL string
	var prometheusTimeout time.Duration
	var prometheusIsAPI bool

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

	pflag.Parse()

	// Initialize Kubernetes client
	clientset, err := initKubeClient(kubeconfig, kubecontext)
	if err != nil {
		fmt.Printf("Error initializing Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// Test the connection by listing nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing nodes: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully connected to Kubernetes API. Found %d nodes.\n", len(nodes.Items))

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

		optimizer := NewOptimizer(scraper, 10*time.Second)

		// Run the optimizer (this will block indefinitely)
		optimizer.Run()
	}
}
