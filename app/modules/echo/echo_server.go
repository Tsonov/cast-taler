package echo

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/spf13/pflag"
)

var (
	listenIP  = pflag.String("echo-server-listen-ip", "0.0.0.0", "IP of echo server")
	keepAlive = pflag.Bool("echo-server-keep-alive", false, "Keep alive connection")
)

type EchoServer struct {
	log   *slog.Logger
	ready *atomic.Bool

	reconnected bool
}

func NewEchoServer(log *slog.Logger, ready *atomic.Bool) *EchoServer {
	return &EchoServer{
		log:   log,
		ready: ready,
	}
}

func (e *EchoServer) Run() error {
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *listenIP, *echoPort))
	if err != nil {
		e.log.Error("Error starting TCP server", Err(err))
		return fmt.Errorf("starting TCP server: %w", err)
	}

	defer listener.Close()
	e.log.Info("Listening", slog.Int("port", *echoPort))

	e.ready.Store(true)

	for {
		conn, err := listener.Accept()
		e.log.Info("Accepted")
		if err != nil {
			e.log.Error("Error accepting connection:", Err(err))
			return fmt.Errorf("accepting connection: %w", err)
		}

		logger := e.log.With(slog.String("client-addr", conn.RemoteAddr().String()))
		go func() {
			err = e.handleConnection(logger, conn)
			if err != nil {
				e.log.Error("Error handling connection", Err(err))
			}
		}()
	}
}

func (e *EchoServer) handleConnection(logger *slog.Logger, conn net.Conn) error {
	defer conn.Close()

	e.log.Info("Start echo data")
	n, err := io.Copy(conn, conn)
	if err != nil {
		logger.Error("Error reading from connection", Err(err))
		return fmt.Errorf("reading from connection: %w", err)
	}

	logger.Info("Done echoing data", slog.Int64("bytes", n))

	return nil
}
