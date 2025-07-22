package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	"github.com/Tsonov/cast-taler/app/modules/echo"
	"github.com/Tsonov/cast-taler/app/pkg/k8s"
	"github.com/Tsonov/cast-taler/app/pkg/metrics"
	"github.com/Tsonov/cast-taler/app/pkg/server"
)

var (
	modules        = pflag.StringSlice("module", nil, "modules to run")
	silent         = pflag.Bool("silent", false, "silence the logger")
	failOnSignal   = pflag.Bool("fail-on-signal", true, "fail on SIGTERM/SIGINT signal")
	readinessPort  = pflag.String("readiness-port", "8081", "port for kubernetes readiness check")
	nodeName       = pflag.String("node-name", "", "name of the node, used for readiness check")
	zoneConfigPath = pflag.String("zone-config-path", "", "path to the zone config file")
)

// startReadinessServer starts an HTTP server for Kubernetes readiness checks
func startReadinessServer(ctx context.Context, logger *slog.Logger, isReady *atomic.Bool) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if isReady.Load() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Not OK"))
	})

	server := &http.Server{
		Addr:    ":" + *readinessPort,
		Handler: mux,
	}

	go func() {
		logger.Info("Starting readiness server on port " + *readinessPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Readiness server failed", log.Err(err))
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down readiness server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func main() {
	pflag.Parse()

	if *silent {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	} else {
		// Set default logger to one that produces `label=value` format even for time and msg so it is understable
		// by the logs matcher in the tests
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	}

	logger := slog.Default().With("module", "main")

	signalCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	availabilityZone := ""
	//TODO: for experimenting with binary locally, remove this check later

	if nodeName != nil && *nodeName != "" {
		k8sClient, err := k8s.NewClient()
		if err != nil {
			logger.Error("Failed to create Kubernetes client", slog.Any("error", err))
			return
		}
		availabilityZone, err = k8s.GetNodeZone(signalCtx, k8sClient, *nodeName)
		if err != nil {
			logger.Error("Failed to get node zone", slog.Any("error", err))
			return
		}
	}

	logger.Info("Node zone", slog.String("zone", availabilityZone))

	zoneConfig, err := server.LoadZoneConfig(*zoneConfigPath)
	if err != nil {
		logger.Error("Failed to load zone config", slog.Any("error", err))
		return
	}

	// Pod name is set as hostname. Since we control the deployment we can be
	// sure it's not set to something else
	podName, err := os.Hostname()
	if err != nil {
		logger.Error("Failed to get hostname", slog.Any("error", err))
		os.Exit(1)
	}

	runGroup, groupCtx := errgroup.WithContext(signalCtx)
	for _, module := range *modules {
		logger := slog.Default().With("module", module)
		switch module {
		case "echo-client":
			runGroup.Go(func() error { return echo.NewEchoClient(logger, availabilityZone, podName).Run(groupCtx) })
		case "echo-server":
			var ready atomic.Bool
			go func() {
				if err := startReadinessServer(context.Background(), logger.With("module", "readiness"), &ready); err != nil {
					panic(err)
				}
			}()
			runGroup.Go(func() error {
				return echo.NewEchoServer(logger, availabilityZone, podName, zoneConfig, &ready).Run(groupCtx)
			})
		}
	}

	runGroup.Go(func() error {
		return startMetricsServer(groupCtx, logger.With("module", "metrics"), 9090)
	})

	outcome := make(chan error)

	go func() {
		outcome <- runGroup.Wait()
		logger.Info("All modules finished")
	}()

	var result error
	select {
	case <-signalCtx.Done():
		if *failOnSignal {
			os.Exit(13)
		}
		select {
		// Give 5 seconds for module to finish
		case <-time.After(time.Second * 5):
			result = signalCtx.Err()
		// Or finish running app
		case err := <-outcome:
			result = err
		}
	case err := <-outcome:
		result = err
	}

	if result != nil {
		logger.Error("Module failed", log.Err(result))
		os.Exit(99)
	}
}

func startMetricsServer(ctx context.Context, logger *slog.Logger, port int) error {
	logger.Info("Starting metrics server")
	addr := fmt.Sprintf(":%d", port)

	metrics.RegisterCustomMetrics()
	metricsMux := metrics.NewMetricsMux()

	server := &http.Server{
		Addr:    addr,
		Handler: metricsMux,
	}

	// Channel to capture server errors
	serverError := make(chan error, 1)

	go func() {
		logger.Info("Metrics server listening", slog.String("address", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- fmt.Errorf("metrics server failed: %w", err)
		}

		serverError <- nil
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-serverError:
		return err
	case <-ctx.Done():
		logger.Info("Shutting down metrics server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}

}
