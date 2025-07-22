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

		err = e.handleConnection(conn)

		if *allowReconnect {
			if !e.reconnected {
				e.reconnected = true
				e.log.Info("Running reconnect")
				continue
			}
		}

		if err != nil {
			e.log.Error("Error handling connection", Err(err))
		}

		if *allowReconnect {
			if e.reconnected {
				e.log.Info("Reconnect successful")
			} else {
				e.log.Error("Didn't handle reconnected connection")
				return fmt.Errorf("didn't handle reconnected connection")
			}
		}

		return nil
	}
}

func (e *EchoServer) handleConnection(conn net.Conn) error {
	defer conn.Close()

	buffer := make([]byte, bufSize)

	iteration := 0
	for {
		n, err := conn.Read(buffer)
		if n == 0 && err != nil {
			if err != io.EOF {
				e.log.Error("Error reading from connection", Err(err))
				return fmt.Errorf("reading from connection: %w", err)
			}

			e.log.Info("Read data from client", slog.Int("bytes", n), slog.Int("iteration", iteration))

			e.log.Info("EOF")
			break
		}

		if iteration%100 == 0 {
			e.log.Info("Read data from client", slog.Int("bytes", n), slog.Int("iteration", iteration))
		}

		nSend, err := conn.Write(buffer[:n])
		if err != nil {
			e.log.Error("Error writing to connection", Err(err))
			return fmt.Errorf("writing to connection: %w", err)
		}

		if iteration%100 == 0 {
			e.log.Info("Send data to client", slog.Int("bytes", nSend), slog.Int("iteration", iteration))
		}
		iteration++
	}

	return nil
}
