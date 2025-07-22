package echo

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"

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

func (e *EchoServer) Run() error {
	http.HandleFunc("/echo", e.handleConnection)
	e.ready.Store(true)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", *listenIP, *echoPort), nil)
	if err != nil {
		e.log.Error("Error starting server", Err(err))
		return fmt.Errorf("starting server: %w", err)
	}
	return nil
}

func (e *EchoServer) handleConnection(writer http.ResponseWriter, request *http.Request) {
	e.log.Info("Start echo data")
	defer request.Body.Close()

	logger := e.log.With(slog.String("client-addr", request.RemoteAddr))
	zoneHeader := request.Header[AvailabilityZoneHeader]
	clientZone := ""
	if len(clientZone) > 0 {
		clientZone = zoneHeader[0]
		logger = e.log.With(slog.String("client-az", clientZone))
	}
	podNameHeader := request.Header[PodNameHeader]
	clientPodName := ""
	if len(clientPodName) > 0 {
		clientPodName = podNameHeader[0]
		logger = e.log.With(slog.String("client-pod-name", clientPodName))
	}

	written, err := io.Copy(writer, request.Body)
	if err != nil {
		logger.Error("Error reading from connection", Err(err))
	}

	returnCode, err := e.zoneConfig.GetRandomCode(e.availabilityZone)
	if err != nil {
		logger.Error("Error getting random code", Err(err))
		return
	}
	writer.WriteHeader(returnCode)
	fmt.Fprintf(writer, "Status code: %d\n", returnCode)

	success := true
	if returnCode != 200 {
		success = false
	}
	metrics.TrackTraffic(
		float64(written), success, "http",
		clientPodName, clientZone,
		e.availabilityZone, e.podName,
	)
	metrics.TrackTraffic(
		float64(written), success, "http",
		e.podName, e.availabilityZone,
		clientZone, clientPodName,
	)
	logger.Info("Done echoing data", slog.Int64("bytes", written))
}
