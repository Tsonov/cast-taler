package echo

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"

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
}

func NewEchoServer(log *slog.Logger, availabilityZone string, zoneConfig *server.ZoneConfig, ready *atomic.Bool) *EchoServer {
	logger := log.With("server-az", availabilityZone)
	return &EchoServer{
		log:              logger,
		availabilityZone: availabilityZone,
		zoneConfig:       zoneConfig,
		ready:            ready,
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
	logger := e.log.With(slog.String("client-addr", request.RemoteAddr))
	zone := request.Header[AvailabilityZoneHeader]
	if len(zone) > 0 {
		logger = e.log.With(slog.String("client-az", zone[0]))
	}
	podName := request.Header[PodNameHeader]
	if len(podName) > 0 {
		logger = e.log.With(slog.String("client-pod-name", podName[0]))
	}

	n, err := io.Copy(writer, request.Body)
	if err != nil {
		logger.Error("Error reading from connection", Err(err))
	}

	logger.Info("Done echoing data", slog.Int64("bytes", n))

	returnCode, err := e.zoneConfig.GetRandomCode(e.availabilityZone)
	if err != nil {
		logger.Error("Error getting random code", Err(err))
		return
	}
	writer.WriteHeader(returnCode)
	fmt.Fprintf(writer, "Status code: %d\n", returnCode)
}
