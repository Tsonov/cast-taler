package echo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Tsonov/cast-taler/app/pkg/metrics"
	"github.com/Tsonov/cast-taler/app/pkg/server"
	"github.com/spf13/pflag"
)

const (
	AvailabilityZoneHeader = "Availability-Zone"
	PodNameHeader          = "Pod-Name"
)

var (
	listenIP  = pflag.String("echo-server-listen-ip", "0.0.0.0", "IP of echo server")
	keepAlive = pflag.Bool("echo-server-keep-alive", false, "Keep alive connection")
)

type EchoServer struct {
	log              *slog.Logger
	availabilityZone string
	zoneConfig       *server.ZoneConfig
	ready            *atomic.Bool
	podName          string
}

func NewEchoServer(log *slog.Logger, availabilityZone string, podName string, zoneConfig *server.ZoneConfig, ready *atomic.Bool) *EchoServer {
	logger := log.With("server-az", availabilityZone)
	return &EchoServer{
		log:              logger,
		availabilityZone: availabilityZone,
		zoneConfig:       zoneConfig,
		ready:            ready,
		podName:          podName,
	}
}

func (e *EchoServer) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", e.handleConnection)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", *listenIP, *echoPort),
		Handler: mux,
	}

	errChan := make(chan error, 1)
	go func() {
		e.ready.Store(true)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("starting server: %w", err)
			return
		}
		errChan <- nil
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutting down server: %w", err)
		}
		return nil
	}
}

func (e *EchoServer) handleConnection(writer http.ResponseWriter, request *http.Request) {
	e.log.Info("Start echo data")
	defer request.Body.Close()

	logger := e.log.With(slog.String("client-addr", request.RemoteAddr))
	zoneHeader := request.Header[AvailabilityZoneHeader]
	clientZone := ""
	if len(zoneHeader) > 0 {
		clientZone = zoneHeader[0]
		logger = e.log.With(slog.String("client-az", clientZone))
	}
	podNameHeader := request.Header[PodNameHeader]
	clientPodName := ""
	if len(podNameHeader) > 0 {
		clientPodName = podNameHeader[0]
		logger = e.log.With(slog.String("client-pod-name", clientPodName))
	}

	zoneSuffix := e.availabilityZone
	if lastHyphen := strings.LastIndex(e.availabilityZone, "-"); lastHyphen >= 0 {
		zoneSuffix = e.availabilityZone[lastHyphen+1:]
	}

	returnCode, err := e.zoneConfig.GetRandomCode(zoneSuffix)
	if err != nil {
		logger.Error("Error getting random code", Err(err))
		return
	}
	writer.WriteHeader(returnCode)

	_, err = fmt.Fprintf(writer, "az: %s\n", e.availabilityZone)
	if err != nil {
		logger.Error("Error reading from connection", Err(err))
	}

	written, err := io.Copy(writer, request.Body)
	if err != nil {
		logger.Error("Error reading from connection", Err(err))
	}
	if returnCode != 200 {
		time.Sleep(1 * time.Second)
	}

	bytesSent := float64(written) * 1000 // increase traffic we report to show nicer numbers

	// egress traffic from the client to the server
	metrics.TrackTraffic(
		bytesSent, true, "http",
		clientPodName, clientZone,
		e.availabilityZone, e.podName,
	)

	success := true
	if returnCode != 200 {
		success = false
		// do not track server egress traffic in case of zone failure simulation
		bytesSent = 0
	}

	// egress traffic from the server to the client
	metrics.TrackTraffic(
		bytesSent, success, "http",
		e.podName, e.availabilityZone,
		clientZone, clientPodName,
	)
	logger.Info("Done echoing data", slog.Int64("bytes", written), slog.Int("status_code", returnCode))
}
